package readercounter

import (
	"io"
	"sync/atomic"
)

// ReaderCounter is counter for io.Reader
type ReaderCounter struct {
	count  int64
	reader io.Reader
}

// New creates ReaderCounter.
func New(r io.Reader) *ReaderCounter {
	return &ReaderCounter{
		reader: r,
	}
}

func (counter *ReaderCounter) Read(buf []byte) (int, error) {
	n, err := counter.reader.Read(buf)
	atomic.AddInt64(&counter.count, int64(n))
	return n, err
}

// Count function return counted bytes
func (counter *ReaderCounter) Count() int64 {
	return atomic.LoadInt64(&counter.count)
}

// Reset resets internal count and reader.
func (counter *ReaderCounter) Reset(r io.Reader) {
	counter.count, counter.reader = 0, r
}
