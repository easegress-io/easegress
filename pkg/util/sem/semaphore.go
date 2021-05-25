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

package sem

import "sync"

const capacity = 2000000

type Semaphore struct {
	sem       uint32
	lock      *sync.Mutex
	guardChan chan *struct{}
}

func NewSem(n uint32) *Semaphore {
	s := &Semaphore{
		sem:       n,
		lock:      &sync.Mutex{},
		guardChan: make(chan *struct{}, capacity),
	}

	go func() {
		for i := uint32(0); i < n; i++ {
			s.guardChan <- &struct{}{}
		}
	}()

	return s
}

func (s *Semaphore) Acquire() {
	<-s.guardChan
}

func (s *Semaphore) AcquireRaw() chan *struct{} {
	return s.guardChan
}

func (s *Semaphore) Release() {
	s.guardChan <- &struct{}{}
}

func (s *Semaphore) SetMaxCount(n uint32) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if n > capacity {
		n = capacity
	}

	if n == s.sem {
		return
	}

	old := s.sem
	s.sem = n

	go func() {
		if n > old {
			for i := uint32(0); i < n-old; i++ {
				s.guardChan <- &struct{}{}
			}
			return
		}

		for i := uint32(0); i < old-n; i++ {
			<-s.guardChan
		}
	}()
}
