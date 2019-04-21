package durationreadcloser

import (
	"io"
	"time"
)

type (
	// DurationReadCloser wraps ReadCloser with duration statistics.
	DurationReadCloser struct {
		rc io.ReadCloser
		d  time.Duration
	}
)

// New creates a DurationReadCloser.
func New(rc io.ReadCloser) *DurationReadCloser {
	return &DurationReadCloser{rc: rc}
}

// Read wraps Read function.
func (d *DurationReadCloser) Read(p []byte) (n int, err error) {
	startTime := time.Now()
	n, err = d.rc.Read(p)
	d.d += time.Now().Sub(startTime)
	return n, err
}

// Close wraps Close.
func (d *DurationReadCloser) Close() error {
	return d.rc.Close()
}

// Duration reports duration of all Read operations.
func (d *DurationReadCloser) Duration() time.Duration {
	return d.d
}
