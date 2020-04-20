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

func nanoToMilli(f float64) uint64 {
	return uint64(f) / 1000000
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

// P50 returns the duration in millisecond greater than 50%.
func (ds *DurationSampler) P50() uint64 {
	return nanoToMilli(ds.sample.Percentile(0.5))
}

// P95 returns the duration in millisecond greater than 95%.
func (ds *DurationSampler) P95() uint64 {
	return nanoToMilli(ds.sample.Percentile(0.95))
}

// P99 returns the duration in millisecond greater than 99%.
func (ds *DurationSampler) P99() uint64 {
	return nanoToMilli(ds.sample.Percentile(0.99))
}

// P50P95P99 wraps other stat functions in calling once.
func (ds *DurationSampler) P50P95P99() (uint64, uint64, uint64) {
	ps := ds.sample.Percentiles([]float64{0.5, 0.95, 0.99})
	return nanoToMilli(ps[0]), nanoToMilli(ps[1]), nanoToMilli(ps[2])
}

// Count return total count of DurationSampler.
func (ds *DurationSampler) Count() uint64 {
	return uint64(ds.sample.Count())
}
