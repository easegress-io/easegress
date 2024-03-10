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

// Package codecounter provides a goroutine unsafe HTTP status code counter.
package codecounter

import "sync/atomic"

// HTTPStatusCodeCounter is the goroutine unsafe HTTP status code counter.
// It is designed for counting http status code which is 1XX - 5XX,
// So the code range are limited to [0, 999]
type HTTPStatusCodeCounter struct {
	counter []uint64
}

// New creates a HTTPStatusCodeCounter.
func New() *HTTPStatusCodeCounter {
	return &HTTPStatusCodeCounter{
		counter: make([]uint64, 1000),
	}
}

// Count counts a new code.
func (cc *HTTPStatusCodeCounter) Count(code int) {
	if code < 0 || code >= len(cc.counter) {
		// TODO: log? panic?
		return
	}
	atomic.AddUint64(&cc.counter[code], 1)
}

// Reset resets counters of all codes to zero
func (cc *HTTPStatusCodeCounter) Reset() {
	for i := 0; i < len(cc.counter); i++ {
		cc.counter[i] = 0
	}
}

// Codes returns the codes.
func (cc *HTTPStatusCodeCounter) Codes() map[int]uint64 {
	codes := make(map[int]uint64)
	for i, count := range cc.counter {
		if count > 0 {
			codes[i] = count
		}
	}
	return codes
}
