package plugins

import (
	"fmt"
	"io"
	"time"

	"github.com/megaease/easegateway/pkg/common"
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
	t.duration += common.Since(start)
}

////

type TimeReader struct {
	*timeIO
	io.Reader
}

func NewTimeReader(r io.Reader) *TimeReader {
	return &TimeReader{
		&timeIO{},
		r,
	}
}

func (tr *TimeReader) Read(p []byte) (n int, err error) { // io.Reader stub
	defer tr.timeTrack(common.Now())
	return tr.Reader.Read(p)
}

type sizedReadClose struct {
	// use embedded type here to expose method of concrete type
	io.ReadCloser
	size int64
}

func NewSizedReadCloser(r io.ReadCloser, s int64) SizedReadCloser {
	return &sizedReadClose{r, s}
}

//
func (l *sizedReadClose) Size() int64 {
	return l.size
}

////
type SizedTimeReader struct {
	// use embedded type to inherit(expose) methods of concrete type
	SizedReadCloser
	// use embedded type to inherit(expose) methods of concrete type
	*timeIO
}

func NewSizedTimeReader(r SizedReadCloser) *SizedTimeReader {
	return &SizedTimeReader{r, &timeIO{}}
}

func (tr *SizedTimeReader) Read(p []byte) (n int, err error) { // io.Reader stub
	defer tr.timeTrack(common.Now())
	return tr.SizedReadCloser.Read(p)
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
	defer tw.timeTrack(common.Now())
	return tw.w.Write(p)
}
