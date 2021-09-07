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
	"bytes"
	"sync"
)

var ioBufferPool IOBufferPool

// IOBufferPool io buffer pool, especially use for udp packet
type IOBufferPool struct {
	pool sync.Pool
}

func (p *IOBufferPool) take() (buf *bytes.Buffer) {
	v := p.pool.Get()
	if v == nil {
		buf = bytes.NewBuffer(nil)
	} else {
		buf = v.(*bytes.Buffer)
	}
	return
}

func (p *IOBufferPool) give(buf *bytes.Buffer) {
	buf.Truncate(0)
	p.pool.Put(buf)
}

// GetIoBuffer returns IoBuffer from pool
func GetIoBuffer() *bytes.Buffer {
	return ioBufferPool.take()
}

func NewIOBuffer() *bytes.Buffer {
	return GetIoBuffer()
}

// PutIoBuffer returns IoBuffer to pool
func PutIoBuffer(buf *bytes.Buffer) error {
	ioBufferPool.give(buf)
	return nil
}
