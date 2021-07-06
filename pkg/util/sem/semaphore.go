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

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

const maxCapacity int64 = 20_000_000

//Semaphore supports to change the max sema amount at runtime.
//  Semaphore employs golang.org/x/sync/semaphore.Weighted with a maxCapacity.
//  And tuning the realCapacity by a Acquire and Release in the background.
//  the realCapacity can not exceeds the maxCapacity.
type Semaphore struct {
	sem          *semaphore.Weighted
	lock         sync.Mutex
	realCapacity int64
}

func NewSem(n uint32) *Semaphore {
	s := &Semaphore{
		sem:          semaphore.NewWeighted(maxCapacity),
		realCapacity: int64(n),
	}

	s.sem.Acquire(context.Background(), maxCapacity-s.realCapacity)
	return s
}

func (s *Semaphore) Acquire() {
	s.AcquireWithContext(context.Background())
}

func (s *Semaphore) AcquireWithContext(ctx context.Context) error {
	return s.sem.Acquire(ctx, 1)
}

func (s *Semaphore) AcquireRaw() chan *struct{} {
	s.AcquireWithContext(context.Background())
	return make(chan *struct{})
}

func (s *Semaphore) Release() {
	s.sem.Release(1)
}

func (s *Semaphore) SetMaxCount(n int64) (done chan struct{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if n > maxCapacity {
		n = maxCapacity
	}

	if n == s.realCapacity {
		return
	}

	old := s.realCapacity
	s.realCapacity = n

	done = make(chan struct{})
	go func() {
		if n > old {
			for i := int64(0); i < n-old; i++ {
				s.sem.Release(1)
			}
		} else {
			for i := int64(0); i < old-n; i++ {
				s.sem.Acquire(context.Background(), 1)
			}
		}

		done <- struct{}{}
	}()

	return done
}
