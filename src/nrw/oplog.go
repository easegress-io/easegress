package nrw

import (
	"common"
	"fmt"
	"logger"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/dgraph-io/badger/badger"
)

const (
	maxSeqKey = "maxSeqKey"
)

// opLog's methods prefixed by underscore(_) can't be invoked by other functions
type opLog struct {
	sync.RWMutex
	kv *badger.KV
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

func (op *opLog) maxSeq() uint64 {
	op.RLock()
	defer op.RUnlock()
	return op._locklessMaxSeq()
}

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
	op.kv.Set([]byte(maxSeqKey), u2b(ms))
	return ms
}

func (op *opLog) append(vals ...[]byte) (uint64, error) {
	op.Lock()
	defer op.Unlock()

	var ms uint64
	for _, val := range vals {
		ms = op._locklessIncreaseMaxSeq()
		op.kv.Set(u2b(ms), val)
	}
	return ms, nil
}

// readLogs reads logs whose sequence is `begin <= seq <= end`
func (op *opLog) readLogs(begin, end uint64) ([][]byte, error) {
	if begin == 0 {
		return nil, fmt.Errorf("begin must be greater than 0")
	}
	if begin > end {
		return nil, fmt.Errorf("begin is greater than end")
	}

	op.RLock()
	defer op.RUnlock()
	ms := op._locklessMaxSeq()
	if end > ms {
		return nil, fmt.Errorf("end is greater than max sequence %d", ms)
	}

	vals := make([][]byte, 0)
	for i := begin; i <= end; i++ {
		val, _ := op.kv.Get(u2b(i))
		if val == nil {
			logger.Errorf("[BUG: read nothing at %d which is less than max sequence %d]", i, ms)
		}
		vals = append(vals, val)
	}

	return vals, nil
}

func (op *opLog) _cleanup() {
	// FIXME: Is it necessary to clean very old values.
}

func (op *opLog) close() {
	op.kv.Close()
}
