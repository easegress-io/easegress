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

package codecounter

// CodeCounter is the goroutine unsafe code counter.
type CodeCounter struct {
	//      code:count
	counter map[int]uint64
}

// New creates a CodeCounter.
func New() *CodeCounter {
	return &CodeCounter{
		counter: make(map[int]uint64),
	}
}

// Count counts a new code.
func (cc *CodeCounter) Count(code int) {
	cc.counter[code]++
}

// Codes returns the codes.
func (cc *CodeCounter) Codes() map[int]uint64 {
	codes := make(map[int]uint64)
	for code, count := range cc.counter {
		codes[code] = count
	}

	return codes
}
