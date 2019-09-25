package ratelimiter

import (
	stdcontext "context"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/fallback"

	metrics "github.com/rcrowley/go-metrics"
	"golang.org/x/time/rate"
)

const (
	// Kind is the kind of RateLimiter.
	Kind = "RateLimiter"

	resultTimeout  = "timeout"
	resultFallback = "fallback"

	maxChanSize = 10000
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         []string{resultTimeout},
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// RateLimiter is the entity to complete rate limiting.
	RateLimiter struct {
		spec *Spec

		fallback *fallback.Fallback

		concurrentGuard chan struct{}
		limiter         *rate.Limiter
		rate1           metrics.EWMA
		done            chan struct{}
	}

	// Spec describes RateLimiter.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		// MaxConcurrent is the max concurrent active requests.
		MaxConcurrent int32          `yaml:"maxConcurrent" v:"lte=10000"`
		TPS           uint32         `yaml:"tps" v:"gte=1"`
		Timeout       string         `yaml:"timeout" v:"omitempty,duration,dmin=1ms"`
		Fallback      *fallback.Spec `yaml:"fallback"`

		timeout *time.Duration
	}

	// Status contains status info of RateLimiter.
	Status struct {
		TPS uint64 `yaml:"tps"`
	}
)

// New creates a RateLimiter.
func New(spec *Spec, prev *RateLimiter) *RateLimiter {
	if spec.Timeout != "" {
		timeout, err := time.ParseDuration(spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse durantion %s failed: %v",
				spec.Timeout, err)
		} else {
			spec.timeout = &timeout
		}
	}

	rl := &RateLimiter{spec: spec}
	if spec.Fallback != nil {
		rl.fallback = fallback.New(spec.Fallback)
	}

	if prev == nil {
		if spec.MaxConcurrent > 0 {
			rl.concurrentGuard = make(chan struct{}, maxChanSize)
			initSize := maxChanSize - spec.MaxConcurrent
			for i := int32(0); i < initSize; i++ {
				rl.concurrentGuard <- struct{}{}
			}
		}

		rl.limiter = rate.NewLimiter(rate.Limit(spec.TPS), 1)
		rl.rate1 = metrics.NewEWMA1()
		rl.done = make(chan struct{})
		go func() {
			for {
				select {
				case <-time.After(5 * time.Second):
					rl.rate1.Tick()
				case <-rl.done:
					return
				}
			}
		}()
		return rl
	}

	switch {
	case prev.concurrentGuard == nil && spec.MaxConcurrent > 0:
		rl.concurrentGuard = make(chan struct{}, maxChanSize)
		initSize := maxChanSize - spec.MaxConcurrent
		for i := int32(0); i < initSize; i++ {
			rl.concurrentGuard <- struct{}{}
		}
	case prev.concurrentGuard != nil && spec.MaxConcurrent > 0:
		rl.concurrentGuard = prev.concurrentGuard
		adjustSize := spec.MaxConcurrent - prev.spec.MaxConcurrent
		switch {
		case adjustSize < 0:
			adjustSize = -adjustSize
			go func() {
				for i := int32(0); i < adjustSize; i++ {
					rl.concurrentGuard <- struct{}{}
				}
			}()
		case adjustSize > 0:
			go func() {
				for i := int32(0); i < adjustSize; i++ {
					<-rl.concurrentGuard
				}
			}()
		}
	case prev.concurrentGuard != nil && spec.MaxConcurrent == 0:
		// Nothing to do.
		// We can't close prev.conccurentGuard in case of panic of running goroutine.
	case prev.concurrentGuard == nil && spec.MaxConcurrent == 0:
		// Nothing to do, just list all possible situations.
	}

	rl.limiter = prev.limiter
	rl.limiter.SetLimit(rate.Limit(spec.TPS))
	rl.rate1 = prev.rate1
	rl.done = prev.done

	return rl
}

// Handle limits HTTPContext.
func (rl *RateLimiter) Handle(ctx context.HTTPContext) string {
	defer rl.rate1.Update(1)

	startTime := time.Now()
	if rl.concurrentGuard != nil {
		timeoutChan := (<-chan time.Time)(nil)
		if rl.spec.timeout != nil {
			timeoutChan = time.After(*rl.spec.timeout)
		}
		select {
		case rl.concurrentGuard <- struct{}{}:
			ctx.OnFinish(func() {
				<-rl.concurrentGuard
			})
		case <-timeoutChan:
			return resultTimeout
		}
	}

	var rlCtx stdcontext.Context = ctx
	if rl.spec.timeout != nil {
		timeout := *rl.spec.timeout
		timeout -= time.Now().Sub(startTime)
		if timeout <= 0 {
			return resultTimeout
		}

		var cancel stdcontext.CancelFunc
		rlCtx, cancel = stdcontext.WithTimeout(rlCtx, timeout)
		defer cancel()
	}

	err := rl.limiter.Wait(rlCtx)
	if err != nil {
		if rl.fallback != nil {
			rl.fallback.Fallback(ctx)
			return resultFallback
		}
		ctx.Response().SetStatusCode(http.StatusTooManyRequests)
		return resultTimeout
	}

	return ""
}

// Status returns RateLimiter status.
func (rl *RateLimiter) Status() *Status {
	return &Status{
		TPS: uint64(rl.rate1.Rate()),
	}
}

// Close closes RateLimiter.
func (rl *RateLimiter) Close() {
	close(rl.done)
}
