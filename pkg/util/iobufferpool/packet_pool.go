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
	"sync"
)

// UDPBufferPool udp buffer pool
var UDPBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, UDPPacketMaxSize)
	},
}

// Packet udp connection msg
type Packet struct {
	Payload []byte
	Len     int
}

// Bytes return underlying bytes for io buffer
func (p *Packet) Bytes() []byte {
	if p.Payload == nil {
		return nil
	}

	return p.Payload[0:p.Len]
}

// Release return io buffer resource to pool
func (p *Packet) Release() {
	if p.Payload == nil {
		return
	}
	UDPBufferPool.Put(p.Payload)
}
