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

	"cluster"
	"common"
)

type opLogAppended func(newOPLog *opLogAtom)

type opLogAtom struct {
	SeqBased uint64
	Type     OperationType
	Content  interface{}
}

var (
	contentConstructorMaps = map[OperationType]func() interface{}{
		createPlugin:   func() interface{} { return new(ContentCreatePlugin) },
		updatePlugin:   func() interface{} { return new(ContentUpdatePlugin) },
		deletePlugin:   func() interface{} { return new(ContentDeletePlugin) },
		createPipeline: func() interface{} { return new(ContentCreatePipeline) },
		updatePipeline: func() interface{} { return new(ContentUpdatePipeline) },
		deletePipeline: func() interface{} { return new(ContentDeletePipeline) },
	}
)

func operation2Atom(operation Operation) (opLogAtom, error) {
	atom := opLogAtom{
		SeqBased: operation.SeqBased,
	}

	if len(operation.Content) < 1 {
		return atom, fmt.Errorf("no content")
	}
	atom.Type = OperationType(operation.Content[0])

	content := contentConstructorMaps[atom.Type]()
	err := cluster.Unpack(operation.Content[1:], content)
	if err != nil {
		return atom, fmt.Errorf("got wrong format: want %T", content)
	}
	atom.Content = content

	return atom, nil
}

func atom2Operation(atom opLogAtom) (Operation, error) {
	operation := Operation{
		SeqBased: atom.SeqBased,
	}
	var err error
	operation.Content, err = cluster.Pack(atom.Content, uint8(atom.Type))
	if err != nil {
		return operation, fmt.Errorf("pack %#v failed: %v", atom.Content, err)
	}

	return operation, nil
}

func marshalAtom(atom opLogAtom) ([]byte, error) {
	buff, err := json.Marshal(atom)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v failed: %v]", atom, err)
	}
	return buff, nil
}

func unmarshalAtom(buff []byte, atom *opLogAtom) error {
	err := json.Unmarshal(buff, atom)
	if err != nil {
		return fmt.Errorf("unmarshal %s failed: %v", buff, err)
	}

	contentBuff, err := json.Marshal(atom.Content)
	if err != nil {
		return fmt.Errorf("marshal content %#v failed: %v", atom.Content, err)
	}

	content := contentConstructorMaps[atom.Type]()
	err = json.Unmarshal(contentBuff, content)
	if err != nil {
		logger.Errorf("unmarshal %s to %T failed: %v", contentBuff, content, err)
		return err
	}
	atom.Content = content

	return nil
}

const (
	maxSeqKey = "maxSeqKey"
)

// TODO: Replace badger with readable text (self-implement maybe).

// opLog's methods prefixed by underscore(_) can't be invoked by other functions
type opLog struct {
	sync.RWMutex
	kv                     *badger.KV
	opLogAppendedCallbacks []*common.NamedCallback
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

func (op *opLog) AddOPLogAppendedCallback(name string, callback opLogAppended, overwrite bool) opLogAppended {
	op.Lock()
	defer op.Unlock()

	var oriCallback interface{}
	op.opLogAppendedCallbacks, oriCallback, _ = common.AddCallback(op.opLogAppendedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(opLogAppended)
	}
}

func (op *opLog) DeleteOPLogAppendedCallback(name string) opLogAppended {
	op.Lock()
	defer op.Unlock()

	var oriCallback interface{}
	op.opLogAppendedCallbacks, oriCallback = common.DeleteCallback(op.opLogAppendedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(opLogAppended)
	}
}

func (op *opLog) maxSeq() uint64 {
	op.RLock()
	defer op.RUnlock()
	return op._locklessMaxSeq()
}

// TODO: use Operation marshal to json and vice versa.

// _locklessMaxSeq is designed to be invoked by locked methods of opLog
func (op *opLog) _locklessMaxSeq() uint64 {
	maxSeq, _ := op.kv.Get([]byte(maxSeqKey))
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
	}

	atoms := make([]opLogAtom, 0)
	for i, operation := range operations {
		atom, err := operation2Atom(operation)
		if err != nil {
			return fmt.Errorf("illegal operation %d: %v", i+1, err)
		}

		atoms = append(atoms, atom)
	}

	for _, atom := range atoms {
		buff, err := marshalAtom(atom)
		if err != nil {
			logger.Errorf("[BUG: marshal %#v failed: %v]", err)
			return fmt.Errorf("marshal %#v failed: %v", atom, err)
		}

		ms := op._locklessIncreaseMaxSeq()
		op.kv.Set([]byte(fmt.Sprintf("%d", ms)), buff)

		for _, callback := range op.opLogAppendedCallbacks {
			callback.Callback().(opLogAppended)(&atom)
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
		buff, _ := op.kv.Get([]byte(fmt.Sprintf("%d", i)))
		if buff == nil {
			logger.Errorf("[BUG: retrieve nothing at %d which is less than max sequence %d]", i, ms)
			return nil, fmt.Errorf("retrieve nothing at %d which is less than max sequence %d", i, ms)
		}

		var atom opLogAtom
		err := unmarshalAtom(buff, &atom)
		if err != nil {
			logger.Errorf("[BUG: at %d unmarshal %s to atom failed: %v]", i, buff, err)
			return nil, fmt.Errorf("at %d unmarshal %s to atom failed: %v", i, buff, err)
		}

		operation, err := atom2Operation(atom)
		if err != nil {
			logger.Errorf("[BUG: at %d transform atom %#v to operation failed: %v]", i, atom, err)
			return nil, fmt.Errorf("at %d transform atom %#v to operation failed: %v", i, atom, err)
		}

		operations = append(operations, operation)
	}

	return operations, nil
}

func (op *opLog) _cleanup() {
	// TODO: Is it necessary to clean very old values?
}

func (op *opLog) close() {
	op.kv.Close()
}
