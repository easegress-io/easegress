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
	BeforeFunc func(num int)

	// AfterFunc runs after each read.
	// num means the number of calling Read, starts from 1.
	AfterFunc func(num int, p []byte, err error)
)

// NewCallbackReader creates CallbackReader.
func NewCallbackReader(r io.Reader) *CallbackReader {
	return &CallbackReader{
		// pre-allocate slices for performance
		beforeFuncs: make([]BeforeFunc, 0, 1),
		afterFuncs:  make([]AfterFunc, 0, 1),
		reader:      r,
	}
}

func (cr *CallbackReader) Read(p []byte) (int, error) {
	cr.num++

	for _, fn := range cr.beforeFuncs {
		fn(cr.num)
	}

	n, err := 0, io.EOF
	if cr.reader != nil {
		n, err = cr.reader.Read(p)
	}

	for _, fn := range cr.afterFuncs {
		fn(cr.num, p[:n], err)
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
