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

// Package sampler provides utilities for sampling.
package sampler

import (
	"sync/atomic"
	"time"
)

type (
	// DurationSampler is the sampler for sampling duration.
	DurationSampler struct {
		count     uint64   // total number of samples
		durations []uint32 // number of samples in each duration
	}

	// DurationSegment defines resolution for a duration segment
	DurationSegment struct {
		resolution time.Duration
		slots      int
	}
)

var segments = []DurationSegment{
	{time.Millisecond, 500},        // < 500ms
	{time.Millisecond * 2, 250},    // < 1s
	{time.Millisecond * 4, 250},    // < 2s
	{time.Millisecond * 8, 125},    // < 3s
	{time.Millisecond * 16, 125},   // < 5s
	{time.Millisecond * 32, 125},   // < 9s
	{time.Millisecond * 64, 125},   // < 17s
	{time.Millisecond * 128, 125},  // < 33s
	{time.Millisecond * 256, 125},  // < 65s
	{time.Millisecond * 512, 125},  // < 129s
	{time.Millisecond * 1024, 125}, // < 257s
}

// NewDurationSampler creates a DurationSampler.
func NewDurationSampler() *DurationSampler {
	slots := 0
	for _, s := range segments {
		slots += s.slots
	}
	// reserved an extra slot for samples which are larger than the maximum
	// duration that the segments can hold.
	return &DurationSampler{durations: make([]uint32, slots+1)}
}

// Update updates the sample. This function could be called concurrently,
// but should not be called concurrently with Percentiles.
func (ds *DurationSampler) Update(d time.Duration) {
	idx := 0
	for _, s := range segments {
		// bound is the upper bound of current segment.
		bound := s.resolution * time.Duration(s.slots)

		// check if d fall into current segment, note that
		// segment is a half-open interval
		if d < bound-s.resolution/2 {
			idx += int((d + s.resolution/2) / s.resolution)
			break
		}
		d -= bound
		idx += s.slots
	}
	atomic.AddUint64(&ds.count, 1)
	atomic.AddUint32(&ds.durations[idx], 1)
}

// Reset reset the DurationSampler to initial state
func (ds *DurationSampler) Reset() {
	for i := 0; i < len(ds.durations); i++ {
		ds.durations[i] = 0
	}
	ds.count = 0
}

// Percentiles returns 7 metrics by order:
// P25, P50, P75, P95, P98, P99, P999
func (ds *DurationSampler) Percentiles() []float64 {
	percentiles := []float64{0.25, 0.5, 0.75, 0.95, 0.98, 0.99, 0.999}

	result := make([]float64, len(percentiles))

	// total is the total number of samples, count is the number of samples
	// we have seen so far.
	count, total := uint64(0), float64(ds.count)

	// no samples, the result is all 0
	if total == 0 {
		return result
	}

	// di is the index of ds.durations, pi is the index of percentiles
	di, pi := 0, 0
	base := time.Duration(0)
	for _, s := range segments {
		for i := 0; i < s.slots; i++ {
			count += uint64(ds.durations[di])
			di++
			// calculate the percentile of samples we have seen against
			// total samples
			p := float64(count) / total

			// fill the result, note one sample may fill multiple percentiles
			for p >= percentiles[pi] {
				d := base + s.resolution*time.Duration(i)
				result[pi] = float64(d / time.Millisecond)
				pi++
				if pi == len(percentiles) {
					return result
				}
			}
		}
		base += s.resolution * time.Duration(s.slots)
	}

	// take samples which are larger than the maximum duration into account,
	// now we have 100% of the samples counted, so we consider all the rest of
	// the result to be the maximum duration (this is not accurate, but we
	// don't have a better solution).
	for pi < len(percentiles) {
		result[pi] = float64(base / time.Millisecond)
		pi++
	}

	return result
}
