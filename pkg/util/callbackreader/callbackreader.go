package callbackreader

import (
	"io"
)

type (
	// CallbackReader is counter for io.Reader
	CallbackReader struct {
		beforeFuncs []BeforeFunc
		afterFuncs  []AfterFunc

		num    int
		reader io.Reader
	}

	// BeforeFunc runs before each Read.
	// num means the number of calling Read, starts from 1.
	BeforeFunc func(num int, p []byte) []byte

	// AfterFunc runs after each read.
	// num means the number of calling Read, starts from 1.
	AfterFunc func(num int, p []byte, n int, err error) ([]byte, int, error)
)

// New creates CallbackReader.
func New(r io.Reader) *CallbackReader {
	return &CallbackReader{
		reader: r,
	}
}

func (cr *CallbackReader) Read(p []byte) (int, error) {
	cr.num++

	for _, fn := range cr.beforeFuncs {
		p = fn(cr.num, p)
	}

	n, err := cr.reader.Read(p)

	for _, fn := range cr.afterFuncs {
		p, n, err = fn(cr.num, p, n, err)
	}

	return n, err
}

// OnBefore registers callback function running before the first read.
func (cr *CallbackReader) OnBefore(fn BeforeFunc) {
	cr.beforeFuncs = append(cr.beforeFuncs, fn)
}

// OnAfter registers callback function running after the last read.
func (cr *CallbackReader) OnAfter(fn AfterFunc) {
	cr.afterFuncs = append(cr.afterFuncs, fn)
}

// Close wraps Close if existed
func (cr *CallbackReader) Close() error {
	closer, ok := cr.reader.(io.Closer)
	if ok {
		return closer.Close()
	}

	return nil
}
