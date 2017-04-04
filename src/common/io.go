package common

import "io"

type InterruptibleReader struct {
	r         io.Reader
	interrupt chan struct{}
}

func NewInterruptibleReader(r io.Reader) *InterruptibleReader {
	return &InterruptibleReader{
		r,
		make(chan struct{}),
	}
}

func (r *InterruptibleReader) Read(p []byte) (int, error) {
	if r.r == nil {
		return 0, io.EOF
	}

	select {
	case <-r.interrupt:
		r.r = nil
		return 0, io.EOF
	default:
		return r.r.Read(p)
	}
}

func (r *InterruptibleReader) Cancel() {
	close(r.interrupt)
}

func (r *InterruptibleReader) Close() {
	close(r.interrupt)
}
