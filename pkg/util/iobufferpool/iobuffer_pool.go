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
	"errors"
	"sync"
)

const (
	UdpPacketMaxSize          = 64 * 1024
	DefaultBufferReadCapacity = 1 << 7
)

var ibPool IoBufferPool

// IoBufferPool is IoBuffer Pool
type IoBufferPool struct {
	pool sync.Pool
}

// take returns IoBuffer from IoBufferPool
func (p *IoBufferPool) take(size int) (buf IoBuffer) {
	v := p.pool.Get()
	if v == nil {
		buf = newIoBuffer(size)
	} else {
		buf = v.(IoBuffer)
		buf.Alloc(size)
		buf.Count(1)
	}
	return
}

// give returns IoBuffer to IoBufferPool
func (p *IoBufferPool) give(buf IoBuffer) {
	buf.Free()
	p.pool.Put(buf)
}

// GetIoBuffer returns IoBuffer from pool
func GetIoBuffer(size int) IoBuffer {
	return ibPool.take(size)
}

// NewIoBuffer is an alias for GetIoBuffer
func NewIoBuffer(size int) IoBuffer {
	return GetIoBuffer(size)
}

// PutIoBuffer returns IoBuffer to pool
func PutIoBuffer(buf IoBuffer) error {
	count := buf.Count(-1)
	if count > 0 {
		return nil
	} else if count < 0 {
		return errors.New("PutIoBuffer duplicate")
	}
	if p, _ := buf.(*pipe); p != nil {
		buf = p.IoBuffer
	}
	ibPool.give(buf)
	return nil
}
