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

// P50 returns the duration greater than 50%.
func (ds *DurationSampler) P50() time.Duration {
	return time.Duration(ds.sample.Percentile(0.5))
}

// P90 returns the duration greater than 90%.
func (ds *DurationSampler) P90() time.Duration {
	return time.Duration(ds.sample.Percentile(0.9))
}

// P95 returns the duration greater than 95%.
func (ds *DurationSampler) P95() time.Duration {
	return time.Duration(ds.sample.Percentile(0.95))
}

// P99 returns the duration greater than 99%.
func (ds *DurationSampler) P99() time.Duration {
	return time.Duration(ds.sample.Percentile(0.99))
}
