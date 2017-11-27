package common

import (
	"io"
	"time"
)

type InterruptibleReader struct {
	r         io.Reader
	interrupt chan struct{}
}

func NewInterruptibleReader(r io.Reader) *InterruptibleReader {
	return &InterruptibleReader{
		r:         r,
		interrupt: make(chan struct{}),
	}
}

func (ir *InterruptibleReader) Read(p []byte) (int, error) {
	if ir.r == nil {
		return 0, io.EOF
	}

	select {
	case <-ir.interrupt:
		ir.r = nil
		return 0, io.EOF
	default:
		return ir.r.Read(p)
	}
}

func (ir *InterruptibleReader) Cancel() {
	close(ir.interrupt)
}

func (ir *InterruptibleReader) Close() {
	close(ir.interrupt)
}

////

type TimeReader struct {
	r        io.Reader
	duration time.Duration
}

func NewTimeReader(r io.Reader) *TimeReader {
	return &TimeReader{
		r: r,
	}
}

func (tr *TimeReader) Read(p []byte) (n int, err error) { // io.Reader stub
	defer tr.timeTrack(time.Now())
	return tr.r.Read(p)
}

func (tr *TimeReader) Elapse() time.Duration {
	return tr.duration
}

func (tr *TimeReader) timeTrack(start time.Time) {
	tr.duration += time.Since(start)
}
