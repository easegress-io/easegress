package common

import (
	"fmt"
	"io"
	"time"
)

var interruptedError = fmt.Errorf("IO interrupted")

type InterruptibleIO struct {
	interrupt chan struct{}
	err       error
}

func newInterruptibleIO() *InterruptibleIO {
	return &InterruptibleIO{
		interrupt: make(chan struct{}),
		err:       interruptedError,
	}
}

func (ii *InterruptibleIO) Cancel(err error) {
	if err != nil {
		ii.err = err
	}
	close(ii.interrupt)
}

func (ii *InterruptibleIO) Close() {
	close(ii.interrupt)
}

////

type InterruptibleReader struct {
	*InterruptibleIO
	r io.Reader
}

func NewInterruptibleReader(r io.Reader) *InterruptibleReader {
	return &InterruptibleReader{
		InterruptibleIO: newInterruptibleIO(),
		r:               r,
	}
}

func (ir *InterruptibleReader) Read(p []byte) (int, error) {
	if ir.r == nil {
		return 0, io.EOF
	}

	select {
	case <-ir.interrupt:
		ir.r = nil
		return 0, ir.err
	default:
		return ir.r.Read(p)
	}
}

////

type InterruptibleWriter struct {
	*InterruptibleIO
	w io.Writer
}

func NewInterruptibleWriter(w io.Writer) *InterruptibleWriter {
	return &InterruptibleWriter{
		InterruptibleIO: newInterruptibleIO(),
		w:               w,
	}
}

func (iw *InterruptibleWriter) Write(p []byte) (n int, err error) {
	if iw.w == nil {
		return 0, io.EOF
	}

	select {
	case <-iw.interrupt:
		iw.w = nil
		return 0, iw.err
	default:
		return iw.w.Write(p)
	}
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
