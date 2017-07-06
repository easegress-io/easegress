package gateway

import (
	"encoding/json"
	"fmt"
	"logger"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/dgraph-io/badger"

	"common"
	"option"
)

type OperationAppended func(seq uint64, newOperation *Operation) (error, OperationFailureType)

const (
	maxSeqKey = "maxSeqKey"
)

// TODO: Replace badger with readable text (self-implement maybe).

// opLog's methods prefixed by underscore(_) can't be invoked by other functions
type opLog struct {
	sync.RWMutex
	kv                         *badger.KV
	operationAppendedCallbacks []*common.NamedCallback
}

func newOPLog() (*opLog, error) {
	dir := filepath.Join(common.INVENTORY_HOME_DIR, "oplog", option.Stage)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		return nil, err
	}

	opt := badger.DefaultOptions
	opt.Dir = dir
	opt.ValueDir = dir
	opt.SyncWrites = true // consistence is more important than performance

	logger.Debugf("[operation logs path: %s]", dir)

	kv, err := badger.NewKV(&opt)
	if err != nil {
		return nil, err
	}

	op := &opLog{
		kv: kv,
	}

	go op._cleanup()

	return op, nil
}

func (op *opLog) maxSeq() uint64 {
	op.RLock()
	defer op.RUnlock()
	return op._locklessMaxSeq()
}

func (op *opLog) append(startSeq uint64, operations []*Operation) (error, ClusterErrorType) {
	if len(operations) == 0 {
		return nil, NoneClusterError
	}

	op.Lock()
	defer op.Unlock()

	ms := op._locklessMaxSeq()

	if startSeq == 0 {
		return fmt.Errorf("invalid sequential operation"), InternalServerError
	} else if startSeq > ms+1 {
		return fmt.Errorf("invalid sequential operation"), OperationInvalidSeqError
	} else if startSeq < ms+1 {
		return fmt.Errorf("operation conflict"), OperationSeqConflictError
	}

	for idx, operation := range operations {
		switch {
		case operation.ContentCreatePlugin != nil:
		case operation.ContentUpdatePlugin != nil:
		case operation.ContentDeletePlugin != nil:
		case operation.ContentCreatePipeline != nil:
		case operation.ContentUpdatePipeline != nil:
		case operation.ContentDeletePipeline != nil:
		default:
			return fmt.Errorf("operation content is empty"), OperationInvalidContentError
		}

		opBuff, err := json.Marshal(operation)
		if err != nil {
			logger.Errorf("[BUG: marshal operation (sequence=%d) %#v failed: %v]",
				startSeq+uint64(idx), operation, err)
			return fmt.Errorf("marshal operation (sequence=%d) %#v failed: %v",
				startSeq+uint64(idx), operation, err), OperationInvalidContentError
		}

		err = op.kv.Set([]byte(fmt.Sprintf("%d", startSeq+uint64(idx))), opBuff)
		if err != nil {
			logger.Errorf("[set operation (sequence=%d) to badger failed: %v]", startSeq+uint64(idx), err)
			return fmt.Errorf("set operation (sequence=%d) to badger failed: %v",
				startSeq+uint64(idx), err), InternalServerError
		}

		// update max sequence at last to keep "transaction" complete
		_, err = op._locklessIncreaseMaxSeq()
		if err != nil {
			logger.Errorf("[update max operation sequence failed: %v]", err)
			return fmt.Errorf("update max operation sequence failed: %v", err), InternalServerError
		}

		for _, cb := range op.operationAppendedCallbacks {
			err, failureType := cb.Callback().(OperationAppended)(startSeq+uint64(idx), operation)
			if err != nil {
				logger.Errorf("[operation (sequence=%d) failed (failure type=%d): %v]",
					startSeq+uint64(idx), failureType, err)

				clusterErrType := InternalServerError

				switch failureType {
				case NoneOperationFailure:
					logger.Errorf("[BUG: operation callback returns error without " +
						"a certain failure type]")
				case OperationGeneralFailure:
					clusterErrType = OperationGeneralFailureError
				case OperationTargetNotFoundFailure:
					clusterErrType = OperationTargetNotFoundFailureError
				case OperationNotAcceptableFailure:
					clusterErrType = OperationNotAcceptableFailureError
				case OperationConflictFailure:
					clusterErrType = OperationConflictFailureError
				case OperationUnknownFailure:
					clusterErrType = OperationUnknownFailureError
				}

				return err, clusterErrType
			}
		}
	}

	return nil, NoneClusterError
}

// retrieve logs whose sequence are [startSeq, MIN(max-sequence, startSeq + countLimit - 1)]
func (op *opLog) retrieve(startSeq, countLimit uint64) ([]*Operation, error, ClusterErrorType) {
	// NOTICE: We never change recorded content, so it's unnecessary to use RLock.
	ms := op._locklessMaxSeq()

	var ret []*Operation

	if startSeq == 0 {
		return nil, fmt.Errorf("invalid begin sequential operation"), InternalServerError
	} else if startSeq > ms {
		return ret, nil, NoneClusterError
	}

	for idx := uint64(0); idx < countLimit && startSeq+uint64(idx) <= op._locklessMaxSeq(); idx++ {
		var item badger.KVItem
		err := op.kv.Get([]byte(fmt.Sprintf("%d", startSeq+uint64(idx))), &item)
		if err != nil {
			logger.Errorf("[get operation (sequence=%d) from badger failed: %v]",
				startSeq+uint64(idx), err)
			return nil, fmt.Errorf("get operation (sequence=%d) from badger failed: %v",
				startSeq+uint64(idx), err), InternalServerError
		}

		opBuff := item.Value()
		if opBuff == nil || len(opBuff) == 0 {
			logger.Errorf("[BUG: get operation (sequence=%d) from badger get empty]",
				startSeq+uint64(idx))
			return nil, fmt.Errorf("get operation (sequence=%d) from badger get empty",
					startSeq+uint64(idx)),
				InternalServerError
		}

		operation := new(Operation)
		err = json.Unmarshal(opBuff, operation)
		if err != nil {
			logger.Errorf("[BUG: unmarshal operation (sequence=%d) %#v failed: %v]",
				startSeq+uint64(idx), opBuff, err)
			return nil, fmt.Errorf("marshal operation (sequence=%d) %#v failed: %v",
				startSeq+uint64(idx), opBuff, err), InternalServerError
		}

		ret = append(ret, operation)
	}

	return ret, nil, NoneClusterError
}

func (op *opLog) close() error {
	return op.kv.Close()
}

////

func (op *opLog) MaxSeq() uint64 {
	return op._locklessMaxSeq()
}

func (op *opLog) AddOPLogAppendedCallback(name string, callback OperationAppended, overwrite bool) OperationAppended {
	op.Lock()
	defer op.Unlock()

	var oriCallback interface{}
	op.operationAppendedCallbacks, oriCallback, _ =
		common.AddCallback(op.operationAppendedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(OperationAppended)
	}
}

func (op *opLog) DeleteOPLogAppendedCallback(name string) OperationAppended {
	op.Lock()
	defer op.Unlock()

	var oriCallback interface{}
	op.operationAppendedCallbacks, oriCallback = common.DeleteCallback(op.operationAppendedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(OperationAppended)
	}
}

////

// _locklessMaxSeq is designed to be invoked by locked methods of opLog
func (op *opLog) _locklessMaxSeq() uint64 {
	var item badger.KVItem
	err := op.kv.Get([]byte(maxSeqKey), &item)
	if err != nil {
		logger.Errorf("[get max sequence from badger failed: %v]", err)
		return 0
	}

	maxSeq := item.Value()
	if maxSeq == nil || len(maxSeq) == 0 {
		// at the beginning, it is not a bug to get empty value.
		maxSeq = []byte("0")
	}

	ms, err := strconv.ParseUint(string(maxSeq), 0, 64)
	if err != nil {
		logger.Errorf("[BUG: parse max sequence %s failed: %s]", string(maxSeq), err)
		return 0
	}

	return ms
}

// _locklessIncreaseMaxSeq is designed to be invoked by locked methods of opLog
func (op *opLog) _locklessIncreaseMaxSeq() (uint64, error) {
	ms := op._locklessMaxSeq()
	ms++

	err := op.kv.Set([]byte(maxSeqKey), []byte(fmt.Sprintf("%d", ms)))
	if err != nil {
		logger.Errorf("[set max sequence to badger failed: %v]", err)
		return 0, err
	}

	return ms, nil
}

func (op *opLog) _cleanup() {
	// TODO: clean very old values
}
