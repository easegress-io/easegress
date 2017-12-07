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

type timeIO struct {
	duration time.Duration
}

func (t *timeIO) Elapse() time.Duration {
	return t.duration
}

func (t *timeIO) timeTrack(start time.Time) {
	t.duration += time.Since(start)
}

////

type TimeReader struct {
	timeIO
	r io.Reader
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

////

type TimeWriter struct {
	timeIO
	w io.Writer
}

func NewTimeWriter(w io.Writer) *TimeWriter {
	return &TimeWriter{
		w: w,
	}
}

func (tw *TimeWriter) Write(p []byte) (n int, err error) { // io.Write stub
	defer tw.timeTrack(time.Now())
	return tw.w.Write(p)
}
