package ratelimiter

import (
	"github.com/megaease/easegateway/pkg/context"
	"golang.org/x/time/rate"
)

type (
	// RateLimiter is the entity to complete rate limiting.
	RateLimiter struct {
		spec    *Spec
		runtime *Runtime
	}

	// Spec describes RateLimiter.
	Spec struct {
		TPS uint32 `yaml:"tps" v:"gte=1"`
	}
)

// New creates a RateLimiter.
func New(spec *Spec, runtime *Runtime) *RateLimiter {
	runtime.limiter.SetLimit(rate.Limit(spec.TPS))
	return &RateLimiter{
		spec:    spec,
		runtime: runtime,
	}
}

// Close closes RateLimiter.
// Nothing to do.
func (rl *RateLimiter) Close() {}

// Limit limits HTTPContext.
func (rl *RateLimiter) Limit(ctx context.HTTPContext) error {
	defer rl.runtime.rate1.Update(1)

	return rl.runtime.limiter.Wait(ctx)
}
