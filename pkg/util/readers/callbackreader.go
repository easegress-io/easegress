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

package readers

import (
	"io"
)

type (
	// CallbackReader wraps an io.Reader, and call the callback functions
	// on some events.
	CallbackReader struct {
		r         io.Reader
		bytesRead int
		err       error

		afterFuncs []AfterFunc
		closeFuncs []CloseFunc
	}

	// AfterFunc will be called after each read.
	//   total is the total bytes have been read.
	//   p is the data that was read by this read.
	//   err is the error when perform this read, could be nil or EOF.
	AfterFunc func(total int, p []byte, err error)

	// CloseFunc will be called when the reader is being closed.
	CloseFunc func()
)

// NewCallbackReader creates CallbackReader.
func NewCallbackReader(r io.Reader) *CallbackReader {
	return &CallbackReader{
		r: r,

		// pre-allocate slices for performance
		afterFuncs: make([]AfterFunc, 0, 1),
		closeFuncs: make([]CloseFunc, 0, 1),
	}
}

func (cr *CallbackReader) Read(p []byte) (int, error) {
	// If there's already an error, return directly to avoid calling
	// the callback functions repeatly.
	if cr.err != nil {
		return 0, cr.err
	}

	n, err := cr.r.Read(p)
	cr.bytesRead += n
	cr.err = err

	for _, fn := range cr.afterFuncs {
		fn(cr.bytesRead, p[:n], err)
	}

	return n, err
}

// OnAfter registers callback function running after the last read.
func (cr *CallbackReader) OnAfter(fn AfterFunc) {
	cr.afterFuncs = append(cr.afterFuncs, fn)
}

// OnClose registers callback function running before the first read.
func (cr *CallbackReader) OnClose(fn CloseFunc) {
	cr.closeFuncs = append(cr.closeFuncs, fn)
}

// Close implements io.Closer.
func (cr *CallbackReader) Close() error {
	for _, fn := range cr.closeFuncs {
		fn()
	}
	if c, ok := cr.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
