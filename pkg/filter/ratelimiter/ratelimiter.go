package ratelimiter

import (
	stdcontext "context"
	"net/http"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
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

var (
	results = []string{resultTimeout, resultFallback}
)

func init() {
	httppipeline.Register(&RateLimiter{})
}

type (
	// RateLimiter is the entity to complete rate limiting.
	RateLimiter struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		fallback *fallback.Fallback

		concurrentGuard chan struct{}
		limiter         *rate.Limiter
		rate1           metrics.EWMA
		done            chan struct{}
	}

	// Spec describes RateLimiter.
	Spec struct {
		// MaxConcurrent is the max concurrent active requests.
		MaxConcurrent int32          `yaml:"maxConcurrent" jsonschema:"omitempty,maximum=10000"`
		TPS           uint32         `yaml:"tps" jsonschema:"required,minimum=1"`
		Timeout       string         `yaml:"timeout" jsonschema:"omitempty,format=duration"`
		Fallback      *fallback.Spec `yaml:"fallback" jsonschema:"omitempty"`

		timeout       *time.Duration
		maxConcurrent int32
	}

	// Status contains status info of RateLimiter.
	Status struct {
		TPS uint64 `yaml:"tps"`
	}
)

// Kind returns the kind of RateLimiter.
func (rl *RateLimiter) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of RateLimiter.
func (rl *RateLimiter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of RateLimiter.
func (rl *RateLimiter) Description() string {
	return "RateLimiter do the rate limiting."
}

// Results returns the results of RateLimiter.
func (rl *RateLimiter) Results() []string {
	return results
}

// Init initializes RateLimiter.
func (rl *RateLimiter) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	rl.pipeSpec, rl.spec, rl.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	rl.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of APIAggregator.
func (rl *RateLimiter) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	rl.pipeSpec, rl.spec, rl.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	rl.reload(previousGeneration.(*RateLimiter))

	// NOTE: Inherited already, can't close here.
	// previousGeneration.Close()
}

func (rl *RateLimiter) reload(previousGeneration *RateLimiter) {
	if rl.spec.Timeout != "" {
		timeout, err := time.ParseDuration(rl.spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse durantion %s failed: %v",
				rl.spec.Timeout, err)
		} else {
			rl.spec.timeout = &timeout
		}
	}

	if rl.spec.MaxConcurrent <= 0 {
		rl.spec.maxConcurrent = maxChanSize
	} else {
		rl.spec.maxConcurrent = rl.spec.MaxConcurrent
	}

	if rl.spec.Fallback != nil {
		rl.fallback = fallback.New(rl.spec.Fallback)
	}

	if previousGeneration == nil {
		rl.concurrentGuard = make(chan struct{}, maxChanSize)
		initSize := maxChanSize - rl.spec.maxConcurrent
		for i := int32(0); i < initSize; i++ {
			rl.concurrentGuard <- struct{}{}
		}

		rl.limiter = rate.NewLimiter(rate.Limit(rl.spec.TPS), 1)
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

		return
	}

	rl.concurrentGuard = previousGeneration.concurrentGuard
	adjustSize := rl.spec.maxConcurrent - previousGeneration.spec.maxConcurrent
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

	rl.limiter = previousGeneration.limiter
	rl.limiter.SetLimit(rate.Limit(rl.spec.TPS))
	rl.rate1 = previousGeneration.rate1
	rl.done = previousGeneration.done
}

// Handle limits HTTPContext.
func (rl *RateLimiter) Handle(ctx context.HTTPContext) (result string) {
	defer func() {
		if result == resultTimeout {
			// NOTE: The HTTPContext will set 499 by itself if client is Disconnected.
			ctx.Response().SetStatusCode(http.StatusTooManyRequests)
			if rl.fallback != nil {
				rl.fallback.Fallback(ctx)
				result = resultFallback
			}
		}

		rl.rate1.Update(1)
	}()

	startTime := time.Now()
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
		return resultTimeout
	}

	return ""
}

// Status returns status.
func (rl *RateLimiter) Status() interface{} {
	return &Status{
		TPS: uint64(rl.rate1.Rate()),
	}
}

// Close closes RateLimiter.
func (rl *RateLimiter) Close() {
	close(rl.done)
}
