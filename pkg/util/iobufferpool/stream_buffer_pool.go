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

package iobufferpool

import (
	"github.com/valyala/bytebufferpool"
)

// StreamBuffer io buffer for stream scene
type StreamBuffer struct {
	payload *bytebufferpool.ByteBuffer
	eof     bool
}

func NewStreamBuffer(buf []byte) *StreamBuffer {
	res := &StreamBuffer{
		payload: bytebufferpool.Get(),
		eof:     false,
	}
	res.payload.Reset()
	_, _ = res.payload.Write(buf)
	return res
}

// NewEOFStreamBuffer create stream buffer with eof sign
func NewEOFStreamBuffer() *StreamBuffer {
	res := &StreamBuffer{
		payload: bytebufferpool.Get(),
		eof:     true,
	}
	res.payload.Reset()
	return res
}

// Bytes return underlying bytes
func (s *StreamBuffer) Bytes() []byte {
	return s.payload.B
}

// Len get buffer len
func (s *StreamBuffer) Len() int {
	return len(s.payload.B)
}

// Write implements io.Writer
func (s *StreamBuffer) Write(p []byte) (int, error) {
	s.payload.B = append(s.payload.B, p...)
	return len(p), nil
}

// Release put buffer resource to pool
func (s *StreamBuffer) Release() {
	if s.payload == nil {
		return
	}
	s.payload.Reset()
	bytebufferpool.Put(s.payload)
}

// EOF return eof sign
func (s *StreamBuffer) EOF() bool {
	return s.eof
}

// SetEOF set eof sign
func (s *StreamBuffer) SetEOF(eof bool) {
	s.eof = eof
}
