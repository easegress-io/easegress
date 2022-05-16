/*
 * Copyright (c) 2017, MegaEase
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

package readers

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
)

var bodyFlushSize = 8 * int64(os.Getpagesize())

// GZipCompressReader wraps an io.Reader to a new io.Reader, whose data
// is the gzip compression result of the original io.Reader.
type GZipCompressReader struct {
	r    io.Reader
	buff *bytes.Buffer
	gw   *gzip.Writer
	err  error
}

// NewGZipCompressReader creates a new GZipCompressReader from r.
func NewGZipCompressReader(r io.Reader) *GZipCompressReader {
	buff := bytes.NewBuffer(nil)
	return &GZipCompressReader{
		r:    r,
		buff: buff,
		gw:   gzip.NewWriter(buff),
	}
}

// Read implements io.Reader.
func (r *GZipCompressReader) Read(p []byte) (n int, err error) {
	for {
		// The error could only be io.EOF, which need to be ignored.
		m, _ := r.buff.Read(p)
		n += m
		if m == len(p) {
			break
		}

		if r.err != nil {
			err = r.err
			break
		}

		r.pull()
		p = p[m:]
	}
	return
}

func (r *GZipCompressReader) pull() {
	// reset the buffer to avoid it becomes too large.
	r.buff.Reset()

	_, r.err = io.CopyN(r.gw, r.r, bodyFlushSize)
	if r.err == io.EOF {
		if err := r.gw.Close(); err != nil {
			r.err = err
		}
	}
}

// Close implements io.Closer and closes the underlying io.Reader if
// it is an io.Closer.
func (r *GZipCompressReader) Close() error {
	if c, ok := r.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
