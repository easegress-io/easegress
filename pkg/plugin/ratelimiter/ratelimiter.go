package ratelimiter

import (
	stdcontext "context"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
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
		TPS     uint32 `yaml:"tps" v:"gte=1"`
		Timeout string `yaml:"timeout" v:"omitempty,duration,dmin=1ms"`

		timeout *time.Duration
	}
)

// New creates a RateLimiter.
func New(spec *Spec, runtime *Runtime) *RateLimiter {
	runtime.limiter.SetLimit(rate.Limit(spec.TPS))

	if spec.Timeout != "" {
		timeout, err := time.ParseDuration(spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse durantion %s failed: %v",
				spec.Timeout, err)
		} else {
			spec.timeout = &timeout
		}
	}

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

	var rlCtx stdcontext.Context = ctx
	if rl.spec.timeout != nil {
		var cancel stdcontext.CancelFunc
		rlCtx, cancel = stdcontext.WithTimeout(rlCtx, *rl.spec.timeout)
		defer cancel()
	}

	return rl.runtime.limiter.Wait(rlCtx)
}
