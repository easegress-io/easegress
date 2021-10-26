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

import "errors"

const (
	// UDPPacketMaxSize max size of udp packet
	UDPPacketMaxSize = 65535
	// DefaultBufferReadCapacity default buffer capacity for stream proxy such as tcp
	DefaultBufferReadCapacity = 1 << 7
)

var (
	// ErrEOF io buffer eof sign
	ErrEOF = errors.New("EOF")
)
