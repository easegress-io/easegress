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

type OperationAppended func(newOperation *Operation)

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
	opt := badger.DefaultOptions
	dir := filepath.Join(common.INVENTORY_HOME_DIR, "/badger_oplogs")
	os.MkdirAll(dir, 0700)
	opt.Dir = dir

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

func (op *opLog) AddOPLogAppendedCallback(name string, callback OperationAppended, overwrite bool) OperationAppended {
	op.Lock()
	defer op.Unlock()

	var oriCallback interface{}
	op.operationAppendedCallbacks, oriCallback, _ = common.AddCallback(op.operationAppendedCallbacks, name, callback, overwrite)

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

func (op *opLog) maxSeq() uint64 {
	op.RLock()
	defer op.RUnlock()
	return op._locklessMaxSeq()
}

// _locklessMaxSeq is designed to be invoked by locked methods of opLog
func (op *opLog) _locklessMaxSeq() uint64 {
	var item badger.KVItem
	err := op.kv.Get([]byte(maxSeqKey), &item)
	if err != nil {
		return 0
	}

	maxSeq := item.Value()
	if maxSeq == nil {
		return 0
	}

	ms, err := strconv.ParseUint(string(maxSeq), 0, 64)
	if err != nil {
		logger.Errorf("[BUG: parse max sequence %s failed: %v]", string(maxSeq), err)
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

func (op *opLog) append(operations ...Operation) error {
	op.Lock()
	defer op.Unlock()

	if len(operations) < 1 {
		return nil
	}

	begin := operations[0].SeqBased
	for _, operation := range operations[1:] {
		if operation.SeqBased-begin != 1 {
			return fmt.Errorf("operations must obey monotonic increasing quantity is 1")
		}
		begin++

		switch {
		case operation.ContentCreatePlugin != nil:
		case operation.ContentUpdatePlugin != nil:
		case operation.ContentDeletePlugin != nil:
		case operation.ContentCreatePipeline != nil:
		case operation.ContentUpdatePipeline != nil:
		case operation.ContentDeletePipeline != nil:
		default:
			return fmt.Errorf("operation with sequence %d has no content", begin)
		}
	}

	for _, operation := range operations {
		operationBuff, err := json.Marshal(operation)
		if err != nil {
			logger.Errorf("[BUG: Marshal %#v failed: %v]", operation, err)
			return fmt.Errorf("[marshal %#v failed: %v]", operation, err)
		}

		ms := op._locklessIncreaseMaxSeq()
		op.kv.Set([]byte(fmt.Sprintf("%d", ms)), operationBuff)

		for _, cb := range op.operationAppendedCallbacks {
			cb.Callback().(OperationAppended)(&operation)
		}
	}

	return nil
}

// retrieve reads logs whose sequence is `begin <= seq <= end`
func (op *opLog) retrieve(begin, end uint64) ([]Operation, error) {
	if begin == 0 {
		return nil, fmt.Errorf("begin must be greater than 0")
	}
	if begin > end {
		return nil, fmt.Errorf("begin is greater than end")
	}

	// op.RLock()
	// defer op.RUnlock()
	// NOTICE: We never change recorded content, so it's unnecessary to use RLock.

	ms := op._locklessMaxSeq()
	if end > ms {
		return nil, fmt.Errorf("end is greater than max sequence %d", ms)
	}

	operations := make([]Operation, 0)
	for i := begin; i <= end; i++ {
		var item badger.KVItem
		err := op.kv.Get([]byte(fmt.Sprintf("%d", i)), &item)
		if err != nil {
			logger.Errorf("[BUG: at %d retrieve operation less than max sequence %d failed: %v]", i, ms, err)
			return nil, fmt.Errorf("[at %d retrieve operation less than max sequence %d failed: %v]", i, ms, err)
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

		operations = append(operations, operation)
	}

	return operations, nil
}

func (op *opLog) _cleanup() {
	// TODO: clean very old values
}

func (op *opLog) close() error {
	return op.kv.Close()
}
