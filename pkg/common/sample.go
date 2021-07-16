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

import (
	"math"
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

// ExpDecaySample is the structure use for monitoring metrics(such as: P90, P50 etc)
type ExpDecaySample struct {
	bucketIdxLock sync.RWMutex
	bucketIdx     uint64
	buckets       []metrics.Sample
	stop          chan struct{}
	closeLock     sync.RWMutex
	closed        bool
}

// NewExpDecaySample creates ExpDecaySample structure
func NewExpDecaySample(timeRange time.Duration, secondsForEachBucket int) *ExpDecaySample {
	if secondsForEachBucket == 0 {
		secondsForEachBucket = int(timeRange / 2)
	}

	s := new(ExpDecaySample)

	n := uint64(math.Ceil(timeRange.Seconds()/float64(secondsForEachBucket))) + 1

	var idx uint64 = 0
	for idx < n {
		s.buckets = append(s.buckets, metrics.NewExpDecaySample(514, 0.015))
		idx++
	}

	s.stop = make(chan struct{})
	switcher := func() {
		ticker := time.NewTicker(time.Second * time.Duration(secondsForEachBucket))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.bucketIdxLock.Lock()

				s.buckets[s.bucketIdx].Clear()
				s.bucketIdx = (s.bucketIdx + 1) % n

				s.bucketIdxLock.Unlock()
			case <-s.stop:
				return
			}
		}
	}

	go switcher()

	return s
}

// Close clean the ExpDecaySample
func (s *ExpDecaySample) Close() {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()

	if s.closed {
		return
	}

	close(s.stop)
	s.closed = true

	// defense, accelerate gc
	for i := 0; i < len(s.buckets); i++ {
		s.buckets[i].Clear()
	}
}

// Update updates the ExpDecaySample
func (s *ExpDecaySample) Update(v int64) {
	for i := 0; i < len(s.buckets); i++ {
		s.buckets[i].Update(v)
	}
}

// Percentile calculates the percentile
func (s *ExpDecaySample) Percentile(q float64) float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Percentile(q)
}

// StdDev return the standard deviation value
func (s *ExpDecaySample) StdDev() float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].StdDev()
}

// Max returns the max value
func (s *ExpDecaySample) Max() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Max()
}

// Min returns the min value
func (s *ExpDecaySample) Min() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Min()
}

// Count returns the count value
func (s *ExpDecaySample) Count() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Count()
}

// Variance returns the variance value
func (s *ExpDecaySample) Variance() float64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Variance()
}

// Sum returns the sum value
func (s *ExpDecaySample) Sum() int64 {
	s.bucketIdxLock.RLock()
	defer s.bucketIdxLock.RUnlock()

	idx := (s.bucketIdx + 1) % uint64(len(s.buckets))

	return s.buckets[idx].Sum()
}
