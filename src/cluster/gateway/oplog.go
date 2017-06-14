package gateway

import (
	"encoding/json"
	"fmt"
	"logger"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/dgraph-io/badger/badger"

	"common"
)

type OperationAppended func(newOperation *Operation) error

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
	dir := filepath.Join(common.INVENTORY_HOME_DIR, "/badger_oplogs")
	os.MkdirAll(dir, 0600)

	opt := badger.DefaultOptions
	opt.Dir = dir
	opt.SyncWrites = true // consistence is more important than performance

	logger.Debugf("[operation logs path: %s]", dir)

	kv, err := badger.NewKV(&opt)
	if err != nil {
		return nil, err
	}

	op := &opLog{
		kv: kv,
	}

	op._checkAndRecovery()

	go op._cleanup()

	return op, nil
}

func (op *opLog) maxSeq() uint64 {
	op.RLock()
	defer op.RUnlock()
	return op._locklessMaxSeq()
}

func (op *opLog) append(operations ...*Operation) (error, ClusterErrorType) {
	if len(operations) == 0 {
		return nil
	}

	op.Lock()
	defer op.Unlock()

	ms := op._locklessMaxSeq()

	// check last one of the sequential operations first to speed up
	if ms >= operations[len(operations)-1].Seq {
		return nil
	}

	leastSeq := operations[0].Seq
	if leastSeq == 0 {
		return fmt.Errorf("invalid sequential operation"), InternalServerError
	}

	if leastSeq != ms + 1 {
		return fmt.Errorf("invalid sequential operation"), OperationWrongSeqError
	}

	leastSeq--

	for _, operation := range operations {
		if operation.Seq-leastSeq != 1 {
			return fmt.Errorf("operation must obey monotonic increasing quantity is 1"),
				OperationWrongSeqError
		}

		leastSeq++

		switch {
		case operation.ContentCreatePlugin != nil:
		case operation.ContentUpdatePlugin != nil:
		case operation.ContentDeletePlugin != nil:
		case operation.ContentCreatePipeline != nil:
		case operation.ContentUpdatePipeline != nil:
		case operation.ContentDeletePipeline != nil:
		default:
			return fmt.Errorf("operation with sequence %d is empty", operation.Seq),
				OperationWrongContentError
		}

		operationBuff, err := json.Marshal(operation)
		if err != nil {
			logger.Errorf("[BUG: marshal %#v failed: %v]", operation, err)
			return fmt.Errorf("[marshal %#v failed: %v]", operation, err), OperationWrongContentError
		}

		op._locklessIncreaseMaxSeq()

		op.kv.Set([]byte(fmt.Sprintf("%d", operation.Seq)), operationBuff)

		for _, cb := range op.operationAppendedCallbacks {
			err := cb.Callback().(OperationAppended)(&operation)
			logger.Errorf("[BUG: operation appended callback %s failed: %v]", cb.Name(), err)
		}
	}

	return nil, NoneError
}

// retrieve reads logs whose sequence is `begin <= seq <= end`
func (op *opLog) retrieve(begin, end uint64) ([]*Operation, error) {
	if begin == 0 {
		return nil, fmt.Errorf("begin must be greater than 0")
	}

	if begin > end {
		return nil, fmt.Errorf("begin is greater than end")
	}

	// NOTICE: We never change recorded content, so it's unnecessary to use RLock.
	ms := op._locklessMaxSeq()

	var ret []*Operation

	for i := begin; i <= end && i <= ms; i++ {
		var item badger.KVItem
		err := op.kv.Get([]byte(fmt.Sprintf("%d", i)), &item)
		if err != nil {
			logger.Errorf("[BUG: at %d retrieve operation less than max sequence %d failed: %v]",
				i, ms, err)
			return nil, fmt.Errorf("[at %d retrieve operation less than max sequence %d failed: %v]",
				i, ms, err)
		}

		operationBuff := item.Value()
		if operationBuff == nil {
			logger.Errorf("[BUG: at %d retrieve nothing less than max sequence %d]", i, ms)
			return nil, fmt.Errorf("at %d retrieve nothing less than max sequence %d", i, ms)
		}

		var operation Operation
		err = json.Unmarshal(operationBuff, &operation)
		if err != nil {
			logger.Errorf("[BUG: at %d unmarshal %s to Operation failed: %v]", i, operationBuff, err)
			return nil, fmt.Errorf("at %d unmarshal %s to Operation failed: %v]", i, operationBuff, err)
		}

		ret = append(ret, &operation)
	}

	return ret, nil
}

func (op *opLog) close() error {
	return op.kv.Close()
}

////

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
	if maxSeq == nil {
		logger.Errorf("[BUG: get max sequence from badger returns empty]")
		return 0
	}

	ms, err := strconv.ParseUint(string(maxSeq), 0, 64)
	if err != nil {
		logger.Errorf("[BUG: parse max sequence %s failed: %s]", string(maxSeq), err)
		return 0
	}

	return ms
}

// _locklessIncreaseMaxSeq is designed to be invoked by locked methods of opLog
func (op *opLog) _locklessIncreaseMaxSeq() uint64 {
	ms := op._locklessMaxSeq()
	ms++
	op.kv.Set([]byte(maxSeqKey), []byte(fmt.Sprintf("%d", ms)))
	return ms
}

func (op *opLog) _checkAndRecovery() {
	// TODO: check the consistence between maxSeqKey and the key of "latest" operation record, and correct maxSeqKey if needed
	// note: we write maxSeqKey+1 first in append(), and badger doesn't support transaction currently
}

func (op *opLog) _cleanup() {
	// TODO: clean very old values
}
