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

package sampler

import (
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

type (
	// DurationSampler is the sampler for sampling duration.
	DurationSampler struct {
		sample metrics.Sample
	}
)

func nanoToMilli(f float64) float64 {
	return f / 1000000
}

// NewDurationSampler creates a DurationSampler.
func NewDurationSampler() *DurationSampler {
	return &DurationSampler{
		// https://github.com/rcrowley/go-metrics/blob/3113b8401b8a98917cde58f8bbd42a1b1c03b1fd/sample_test.go#L65
		sample: metrics.NewExpDecaySample(1028, 0.015),
	}
}

// Update updates the sample.
func (ds *DurationSampler) Update(d time.Duration) {
	ds.sample.Update(int64(d))
}

// P25 returns the duration in millisecond greater than 25%.
func (ds *DurationSampler) P25() float64 {
	return nanoToMilli(ds.sample.Percentile(0.25))
}

// P50 returns the duration in millisecond greater than 50%.
func (ds *DurationSampler) P50() float64 {
	return nanoToMilli(ds.sample.Percentile(0.5))
}

// P75 returns the duration in millisecond greater than 75%.
func (ds *DurationSampler) P75() float64 {
	return nanoToMilli(ds.sample.Percentile(0.75))
}

// P95 returns the duration in millisecond greater than 95%.
func (ds *DurationSampler) P95() float64 {
	return nanoToMilli(ds.sample.Percentile(0.95))
}

// P98 returns the duration in millisecond greater than 98%.
func (ds *DurationSampler) P98() float64 {
	return nanoToMilli(ds.sample.Percentile(0.98))
}

// P99 returns the duration in millisecond greater than 99%.
func (ds *DurationSampler) P99() float64 {
	return nanoToMilli(ds.sample.Percentile(0.99))
}

// P999 returns the duration in millisecond greater than 99.9%.
func (ds *DurationSampler) P999() float64 {
	return nanoToMilli(ds.sample.Percentile(0.999))
}

// Percentiles returns 7 metrics by order:
// P25, P50, P75, P95, P98, P99, P999
func (ds *DurationSampler) Percentiles() []float64 {
	ps := ds.sample.Percentiles([]float64{
		0.25, 0.5, 0.75,
		0.95, 0.98, 0.99,
		0.999,
	})
	for i, p := range ps {
		ps[i] = nanoToMilli(p)
	}

	return ps
}

// Count return total count of DurationSampler.
func (ds *DurationSampler) Count() float64 {
	return float64(ds.sample.Count())
}
