package timelimiter

import (
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/resilience"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of TimeLimiter.
	Kind              = "TimeLimiter"
	resultTimeLimiter = "timeLimiter"
)

var (
	results = []string{resultTimeLimiter}
)

func init() {
	httppipeline.Register(&TimeLimiter{})
}

type (
	URLRule struct {
		resilience.URLRule `yaml:",inline"`
		TimeoutDuration    string `yaml:"timeoutDuration" jsonschema:"omitempty,format=duration"`
		timeout            time.Duration
	}

	Spec struct {
		DefaultTimeoutDuration string `yaml:"defaultTimeoutDuration" jsonschema:"omitempty,format=duration"`
		defaultTimeout         time.Duration
		URLs                   []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	TimeLimiter struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}
)

// Kind returns the kind of TimeLimiter.
func (tl *TimeLimiter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TimeLimiter.
func (tl *TimeLimiter) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of TimeLimiter
func (tl *TimeLimiter) Description() string {
	return "TimeLimiter implements a time limiter for http request."
}

// Results returns the results of TimeLimiter.
func (tl *TimeLimiter) Results() []string {
	return results
}

// Init initializes TimeLimiter.
func (tl *TimeLimiter) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	tl.pipeSpec = pipeSpec
	tl.spec = pipeSpec.FilterSpec().(*Spec)
	tl.super = super

	if d := tl.spec.DefaultTimeoutDuration; d != "" {
		tl.spec.defaultTimeout, _ = time.ParseDuration(d)
	} else {
		tl.spec.defaultTimeout = 500 * time.Millisecond
	}

	for _, url := range tl.spec.URLs {
		if d := url.TimeoutDuration; d != "" {
			url.timeout, _ = time.ParseDuration(d)
		} else {
			url.timeout = tl.spec.defaultTimeout
		}
	}
}

// Inherit inherits previous generation of TimeLimiter.
func (tl *TimeLimiter) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	tl.Init(pipeSpec, super)
}

func (tl *TimeLimiter) handle(ctx context.HTTPContext, u *URLRule) string {
	wrapper := func(fn context.HandlerFunc) context.HandlerFunc {
		return func() string {
			var result string
			timer := time.NewTimer(u.timeout)
			ch := make(chan struct{})
			go func() {
				result = fn()
				ch <- struct{}{}
			}()

			select {
			case <-ch:
				return result
			case <-timer.C:
				logger.Infof("request to URL(%s) timed out", u.ID())
				return resultTimeLimiter
			}
		}
	}

	ctx.AddHandlerWrapper("timeLimiter", wrapper)
	return ""
}

// Handle handles HTTP request
func (tl *TimeLimiter) Handle(ctx context.HTTPContext) string {
	for _, u := range tl.spec.URLs {
		if u.Match(ctx.Request()) {
			return tl.handle(ctx, u)
		}
	}
	return ""
}

// Status returns Status genreated by Runtime.
func (tl *TimeLimiter) Status() interface{} {
	return nil
}

// Close closes TimeLimiter.
func (tl *TimeLimiter) Close() {
}
