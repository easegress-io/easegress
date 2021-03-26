package circuitbreaker

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/filter/resilience"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/supervisor"
	libcb "github.com/megaease/easegateway/pkg/util/circuitbreaker"
)

const (
	// Kind is the kind of CircuitBreaker.
	Kind                 = "CircuitBreaker"
	resultCircuitBreaker = "circuitBreaker"
)

var (
	results = []string{resultCircuitBreaker}
)

func init() {
	httppipeline.Register(&CircuitBreaker{})
}

type (
	// Policy defines the policy of a circuit breaker
	Policy struct {
		Name                             string `yaml:"name" jsonschema:"required"`
		SlidingWindowType                string `yaml:"slidingWindowType" jsonschema:"omitempty" jsonschema:"omitempty,enum=COUNT_BASED,enum=TIME_BASED"`
		FailureRateThreshold             uint8  `yaml:"failureRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
		SlowCallRateThreshold            uint8  `yaml:"slowCallRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
		SlidingWindowSize                uint32 `yaml:"slidingWindowSize" jsonschema:"omitempty,minimum=1"`
		PermittedNumberOfCallsInHalfOpen uint32 `yaml:"permittedNumberOfCallsInHalfOpenState" jsonschema:"omitempty"`
		MinimumNumberOfCalls             uint32 `yaml:"minimumNumberOfCalls" jsonschema:"omitempty"`
		SlowCallDurationThreshold        string `yaml:"slowCallDurationThreshold" jsonschema:"omitempty,format=duration"`
		MaxWaitDurationInHalfOpen        string `yaml:"maxWaitDurationInHalfOpenState" jsonschema:"omitempty,format=duration"`
		WaitDurationInOpen               string `yaml:"waitDurationInOpenState" jsonschema:"omitempty,format=duration"`
	}

	// URLRule defines the circuit breaker rule for a URL pattern
	URLRule struct {
		resilience.URLRule `yaml:",inline"`
		policy             *Policy
		cb                 *libcb.CircuitBreaker
	}

	// Spec is the configuration of a circuit breaker
	Spec struct {
		Policies         []*Policy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string     `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*URLRule `yaml:"urls" jsonschema:"required"`
	}

	// CircuitBreaker defines the circuit breaker
	CircuitBreaker struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec
	}

	// Status is the status of CircuitBreaker.
	Status struct {
		Health string `yaml:"health"`
	}
)

// Validate implements custom validation for Spec
func (spec Spec) Validate() error {
URLLoop:
	for _, u := range spec.URLs {
		name := u.PolicyRef
		if name == "" {
			name = spec.DefaultPolicyRef
		}

		for _, p := range spec.Policies {
			if p.Name == name {
				continue URLLoop
			}
		}

		return fmt.Errorf("policy '%s' is not defined", name)
	}

	return nil
}

func (url *URLRule) createCircuitBreaker() {
	policy := libcb.Policy{
		FailureRateThreshold:             url.policy.FailureRateThreshold,
		SlowCallRateThreshold:            url.policy.SlowCallRateThreshold,
		SlidingWindowType:                libcb.CountBased,
		SlidingWindowSize:                url.policy.SlidingWindowSize,
		PermittedNumberOfCallsInHalfOpen: url.policy.PermittedNumberOfCallsInHalfOpen,
		MinimumNumberOfCalls:             url.policy.MinimumNumberOfCalls,
	}

	if policy.FailureRateThreshold == 0 {
		policy.FailureRateThreshold = 50
	}

	if policy.SlowCallRateThreshold == 0 {
		policy.SlowCallRateThreshold = 100
	}

	if strings.ToUpper(url.policy.SlidingWindowType) == "TIME_BASED" {
		policy.SlidingWindowType = libcb.TimeBased
	}

	if policy.SlidingWindowSize == 0 {
		policy.SlidingWindowSize = 100
	}

	if policy.PermittedNumberOfCallsInHalfOpen == 0 {
		policy.PermittedNumberOfCallsInHalfOpen = 10
	}

	if policy.MinimumNumberOfCalls == 0 {
		policy.MinimumNumberOfCalls = 100
	}

	if d := url.policy.SlowCallDurationThreshold; d != "" {
		policy.SlowCallDurationThreshold, _ = time.ParseDuration(d)
	} else {
		policy.SlowCallDurationThreshold = time.Minute
	}

	if d := url.policy.MaxWaitDurationInHalfOpen; d != "" {
		policy.MaxWaitDurationInHalfOpen, _ = time.ParseDuration(d)
	}

	if d := url.policy.WaitDurationInOpen; d != "" {
		policy.WaitDurationInOpen, _ = time.ParseDuration(d)
	} else {
		policy.WaitDurationInOpen = time.Minute
	}

	url.cb = libcb.New(&policy)
}

// Kind returns the kind of CircuitBreaker.
func (cb *CircuitBreaker) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of CircuitBreaker.
func (cb *CircuitBreaker) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of CircuitBreaker
func (cb *CircuitBreaker) Description() string {
	return "CircuitBreaker implements a circuit breaker for http request."
}

// Results returns the results of CircuitBreaker.
func (cb *CircuitBreaker) Results() []string {
	return results
}

func (cb *CircuitBreaker) setStateListenerForURL(u *URLRule) {
	u.cb.SetStateListener(func(event *libcb.Event) {
		logger.Infof("state of circuit breaker '%s' on URL(%s) transited from %s to %s at %d, reason: %s",
			cb.pipeSpec.Name(),
			u.ID(),
			event.OldState,
			event.NewState,
			event.Time.UnixNano()/1e6,
			event.Reason,
		)
	})
}

func (cb *CircuitBreaker) createCircuitBreakerForURL(u *URLRule) {
	u.Init()

	name := u.PolicyRef
	if name == "" {
		name = cb.spec.DefaultPolicyRef
	}

	for _, p := range cb.spec.Policies {
		if p.Name == name {
			u.policy = p
			break
		}
	}

	u.createCircuitBreaker()
	cb.setStateListenerForURL(u)
}

func isSamePolicy(spec1, spec2 *Spec, policyName string) bool {
	if policyName == "" {
		if spec1.DefaultPolicyRef != spec2.DefaultPolicyRef {
			return false
		}
		policyName = spec1.DefaultPolicyRef
	}

	var p1, p2 *Policy
	for _, p := range spec1.Policies {
		if p.Name == policyName {
			p1 = p
			break
		}
	}

	for _, p := range spec2.Policies {
		if p.Name == policyName {
			p2 = p
			break
		}
	}

	return reflect.DeepEqual(p1, p2)
}

func (cb *CircuitBreaker) reload(previousGeneration *CircuitBreaker) {
	if previousGeneration == nil {
		for _, u := range cb.spec.URLs {
			cb.createCircuitBreakerForURL(u)
		}
		return
	}

OuterLoop:
	for _, url := range cb.spec.URLs {
		for _, prev := range previousGeneration.spec.URLs {
			if !url.DeepEqual(&prev.URLRule) {
				continue
			}
			if !isSamePolicy(cb.spec, previousGeneration.spec, url.PolicyRef) {
				continue
			}

			url.Init()
			url.cb = prev.cb
			prev.cb = nil
			cb.setStateListenerForURL(url)
			continue OuterLoop
		}
		cb.createCircuitBreakerForURL(url)
	}
}

// Init initializes CircuitBreaker.
func (cb *CircuitBreaker) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	cb.pipeSpec = pipeSpec
	cb.spec = pipeSpec.FilterSpec().(*Spec)
	cb.super = super
	cb.reload(nil)
}

// Inherit inherits previous generation of CircuitBreaker.
func (cb *CircuitBreaker) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {
	cb.pipeSpec = pipeSpec
	cb.spec = pipeSpec.FilterSpec().(*Spec)
	cb.super = super
	cb.reload(previousGeneration.(*CircuitBreaker))
}

func (cb *CircuitBreaker) handle(ctx context.HTTPContext, u *URLRule) string {
	permitted, stateID := u.cb.AcquirePermission()
	if !permitted {
		return ctx.CallNextHandler(resultCircuitBreaker)
	}

	start := time.Now()
	defer func() {
		if e := recover(); e != nil {
			d := time.Since(start)
			u.cb.RecordResult(stateID, true, d)
			panic(e)
		}
	}()

	result := ctx.CallNextHandler("")
	d := time.Since(start)
	u.cb.RecordResult(stateID, result != "", d)

	return result
}

// Handle handles HTTP request
func (cb *CircuitBreaker) Handle(ctx context.HTTPContext) string {
	for _, u := range cb.spec.URLs {
		if u.Match(ctx.Request()) {
			return cb.handle(ctx, u)
		}
	}
	return ctx.CallNextHandler("")
}

// Status returns Status genreated by Runtime.
func (cb *CircuitBreaker) Status() interface{} {
	return nil
}

// Close closes CircuitBreaker.
func (cb *CircuitBreaker) Close() {
}
