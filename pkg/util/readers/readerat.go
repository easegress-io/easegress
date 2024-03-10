/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package readers provides several readers.
package readers

import (
	"io"
	"sync"
	"sync/atomic"
)

// ReaderAt implements io.ReaderAt, io.Closer on the underlying io.Reader.
// Note Close of ReaderAt calls Close of the underlying io.Reader, if the
// io.Reader implements io.Closer.
type ReaderAt struct {
	lock   sync.Mutex
	r      io.Reader
	seeEOF bool
	buf    atomic.Value
}

// NewReaderAt returns a ReaderAt that reads from r.
func NewReaderAt(r io.Reader) *ReaderAt {
	return &ReaderAt{r: r}
}

func (ra *ReaderAt) buffer() []byte {
	p := ra.buf.Load()
	if p == nil {
		return nil
	}
	return p.([]byte)
}

func (ra *ReaderAt) growBuffer(newSize int) error {
	ra.lock.Lock()
	defer ra.lock.Unlock()

	// when we waiting for the lock, other goroutines may have already
	// done what we need.
	buf := ra.buffer()
	size := len(buf)
	if size > newSize {
		return nil
	}
	if ra.seeEOF {
		return io.EOF
	}

	// grow the buffer to newSize.
	if cap(buf) >= newSize {
		buf = buf[:newSize]
	} else {
		newBuf := make([]byte, newSize, newSize*2)
		copy(newBuf, buf)
		buf = newBuf
	}

	// read from the underlying reader.
	n, err := ra.r.Read(buf[size:])
	if err == io.EOF {
		ra.seeEOF = true
	}

	// len(buf) is newSize, but it is possible that size + n < newSize.
	buf = buf[:size+n]
	ra.buf.Store(buf)
	return err
}

// ReadAt implements io.ReaderAt.
func (ra *ReaderAt) ReadAt(p []byte, off int64) (int, error) {
	buf := ra.buffer()
	if int(off)+len(p) <= len(buf) {
		return copy(p, buf[off:]), nil
	}

	err := ra.growBuffer(int(off) + len(p))
	// must get the buffer again, growBuffer may change it.
	buf = ra.buffer()
	return copy(p, buf[off:]), err
}

// Close implements io.Closer.
func (ra *ReaderAt) Close() error {
	if ra.r == nil {
		return nil
	}
	if c, ok := ra.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// ReaderAtReader implements io.Reader on the underlying io.ReaderAt.
type ReaderAtReader struct {
	ra  io.ReaderAt
	off int64
}

// NewReaderAtReader returns a ReaderAtReader that reads from offset off of ra.
func NewReaderAtReader(ra io.ReaderAt, off int64) *ReaderAtReader {
	return &ReaderAtReader{
		ra:  ra,
		off: off,
	}
}

// Read implements io.Reader
func (rar *ReaderAtReader) Read(p []byte) (int, error) {
	n, err := rar.ra.ReadAt(p, rar.off)
	rar.off += int64(n)
	return n, err
}
