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

package common

// NOTE: Copy from path/path.go in standard library.

// A Lazybuf is a lazily constructed path buffer.
// It supports append, reading previously appended bytes,
// and retrieving the final string. It does not allocate a buffer
// to hold the output until that output diverges from s.
type Lazybuf struct {
	s   string
	buf []byte
	w   int
}

// NewLazybuf returns lazy buffer
func NewLazybuf(s string) *Lazybuf {
	return &Lazybuf{
		s: s,
	}
}

// Index return a byte by its index
func (b *Lazybuf) Index(i int) byte {
	if b.buf != nil {
		return b.buf[i]
	}
	return b.s[i]
}

// Append appends a byte into buffer
func (b *Lazybuf) Append(c byte) {
	if b.buf == nil {
		if b.w < len(b.s) && b.s[b.w] == c {
			b.w++
			return
		}
		b.buf = make([]byte, len(b.s))
		copy(b.buf, b.s[:b.w])
	}
	b.buf[b.w] = c
	b.w++
}

// String return the string
func (b *Lazybuf) String() string {
	if b.buf == nil {
		return b.s[:b.w]
	}
	return string(b.buf[:b.w])
}
