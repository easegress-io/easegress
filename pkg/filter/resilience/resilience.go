package resilience

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Kind is the kind of Resilience.
	Kind = "Resilience"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&Resilience{})
}

type (
	// Resilience is Object Resilience.
	Resilience struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		circuitBreaker *CircuitBreaker
		rateLimiter    *RateLimiter
		retryer        *Retryer
		timeLimiter    *TimeLimiter
	}

	// Spec describes the Resilience.
	Spec struct {
		CircuitBreaker *CircuitBreakerSpec `yaml:"circuitBreaker,omitempty" jsonschema:"omitempty"`
		RateLimiter    *RateLimiterSpec    `yaml:"rateLimiter,omitempty" jsonschema:"omitempty"`
		Retryer        *RetryerSpec        `yaml:"retryer,omitempty" jsonschema:"omitempty"`
		TimeLimiter    *TimeLimiterSpec    `yaml:"timeLimiter,omitempty" jsonschema:"omitempty"`
	}

	// Status is the status of Resilience.
	Status struct {
		Health string `yaml:"health"`
	}
)

// Validate implements custom validation for Spec
func (spec Spec) Validate() error {
	if spec.CircuitBreaker != nil {
		return nil
	}

	if spec.RateLimiter != nil {
		return nil
	}

	if spec.Retryer != nil {
		return nil
	}

	if spec.TimeLimiter != nil {
		return nil
	}

	return fmt.Errorf("at least one resilience method must be configured")
}

// Kind returns the kind of Resilience.
func (r *Resilience) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Resilience.
func (r *Resilience) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Resilience
func (r *Resilience) Description() string {
	return "Resilience provides resilience for http request."
}

// Results returns the results of Resilience.
func (r *Resilience) Results() []string {
	return results
}

func (r *Resilience) reload() {
	if r.spec.CircuitBreaker != nil {
		r.circuitBreaker = NewCircuitBreaker(r.spec.CircuitBreaker)
	}

	if r.spec.RateLimiter != nil {
		r.rateLimiter = NewRateLimiter(r.spec.RateLimiter)
	}

	if r.spec.Retryer != nil {
		r.retryer = NewRetryer(r.spec.Retryer)
	}

	if r.spec.TimeLimiter != nil {
		r.timeLimiter = NewTimeLimiter(r.spec.TimeLimiter)
	}
}

// Init initializes Resilience.
func (r *Resilience) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	r.pipeSpec = pipeSpec
	r.spec = pipeSpec.FilterSpec().(*Spec)
	r.super = super
	r.reload()
}

// Inherit inherits previous generation of Resilience.
func (r *Resilience) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	previousGeneration.Close()
	r.Init(pipeSpec, super)
}

// Handle handles one HTTP request
func (r *Resilience) Handle(ctx context.HTTPContext) string {
	if r.circuitBreaker != nil {
		if result := r.circuitBreaker.Handle(ctx); result != "" {
			return result
		}
	}

	if r.rateLimiter != nil {
		if result := r.rateLimiter.Handle(ctx); result != "" {
			return result
		}
	}

	if r.retryer != nil {
		if result := r.retryer.Handle(ctx); result != "" {
			return result
		}
	}

	if r.timeLimiter != nil {
		if result := r.timeLimiter.Handle(ctx); result != "" {
			return result
		}
	}

	return ""
}

// Status returns Status genreated by Runtime.
func (r *Resilience) Status() interface{} {
	return nil
}

// Close closes Resilience.
func (r *Resilience) Close() {
}
