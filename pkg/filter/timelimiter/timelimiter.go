package timelimiter

import (
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of TimeLimiter.
	Kind = "TimeLimiter"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&TimeLimiter{})
}

type (
	Policy struct {
		Name               string
		MaxAttempts        int
		WaitDuration       int
		BackOffPolicy      string
		RandomWaitInterval float64
	}

	URLRule struct {
		// URLRule
		policy *Policy
	}

	Spec struct {
		Policies         []*Policy
		DefaultPolicyRef string
		URLs             []URLRule
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

func (tl *TimeLimiter) reload() {
}

// Init initializes TimeLimiter.
func (tl *TimeLimiter) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	tl.pipeSpec = pipeSpec
	tl.spec = pipeSpec.FilterSpec().(*Spec)
	tl.super = super
	tl.reload()
}

// Inherit inherits previous generation of TimeLimiter.
func (tl *TimeLimiter) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	previousGeneration.Close()
	tl.Init(pipeSpec, super)
}

// Handle handles HTTP request
func (tl *TimeLimiter) Handle(ctx context.HTTPContext) string {
	return ""
}

// Status returns Status genreated by Runtime.
func (tl *TimeLimiter) Status() interface{} {
	return nil
}

// Close closes TimeLimiter.
func (tl *TimeLimiter) Close() {
}
