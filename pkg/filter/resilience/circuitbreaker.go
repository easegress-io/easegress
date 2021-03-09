package resilience

import (
	"fmt"
	"sort"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/circuitbreaker"
)

type (
	// CircuitBreakerPolicy defines the policy of a circuit breaker
	CircuitBreakerPolicy struct {
		Name                             string        `yaml:"name" jsonschema:"required"`
		SlidingWindowType                string        `yaml:"slidingWindowType" jsonschema:"omitempty" jsonschema:"omitempty,enum=COUNT_BASED,enum=TIME_BASED"`
		CountingNetworkException         bool          `yaml:"countingNetworkException"`
		FailureRateThreshold             uint8         `yaml:"failureRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
		SlowCallRateThreshold            uint8         `yaml:"slowCallRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
		SlidingWindowSize                uint32        `yaml:"slidingWindowSize" jsonschema:"omitempty,minimum=1"`
		PermittedNumberOfCallsInHalfOpen uint32        `yaml:"permittedNumberOfCallsInHalfOpenState" jsonschema:"omitempty"`
		MinimumNumberOfCalls             uint32        `yaml:"minimumNumberOfCalls" jsonschema:"omitempty"`
		SlowCallDurationThreshold        time.Duration `yaml:"slowCallDurationThreshold" jsonschema:"omitempty,format=duration"`
		MaxWaitDurationInHalfOpen        time.Duration `yaml:"maxWaitDurationInHalfOpenState" jsonschema:"omitempty,format=duration"`
		WaitDurationInOpen               time.Duration `yaml:"waitDurationInOpenState" jsonschema:"omitempty,format=duration"`
		ExceptionalStatusCode            []int         `yaml:"exceptionalStatusCode" jsonschema:"omitempty,uniqueItems=true"`
	}

	// CircuitBreakerURLRule defines the circuit breaker rule for a URL pattern
	CircuitBreakerURLRule struct {
		URLRule
		policy *CircuitBreakerPolicy
		cb     *circuitbreaker.CircuitBreaker
	}

	// CircuitBreakerSpec is the configuration of a circuit breaker
	CircuitBreakerSpec struct {
		Policies         []*CircuitBreakerPolicy  `yaml:"policies" jsonschema:"required"`
		DefaultPolicyRef string                   `yaml:"defaultPolicyRef" jsonschema:"omitempty"`
		URLs             []*CircuitBreakerURLRule `yaml:"urls" jsonschema:"required"`
	}

	// CircuitBreaker defines the circuit breaker
	CircuitBreaker struct {
		spec *CircuitBreakerSpec
	}
)

// Validate implements custom validation for CircuitBreakerSpec
func (spec *CircuitBreakerSpec) Validate() error {
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

func (url *CircuitBreakerURLRule) createCircuitBreaker() {
	policy := circuitbreaker.Policy{
		FailureRateThreshold:             url.policy.FailureRateThreshold,
		SlowCallRateThreshold:            url.policy.SlowCallRateThreshold,
		SlidingWindowType:                circuitbreaker.CountBased,
		SlidingWindowSize:                url.policy.SlidingWindowSize,
		PermittedNumberOfCallsInHalfOpen: url.policy.PermittedNumberOfCallsInHalfOpen,
		MinimumNumberOfCalls:             url.policy.MinimumNumberOfCalls,
		SlowCallDurationThreshold:        url.policy.SlowCallDurationThreshold,
		MaxWaitDurationInHalfOpen:        url.policy.MaxWaitDurationInHalfOpen,
		WaitDurationInOpen:               url.policy.WaitDurationInOpen,
	}

	if policy.FailureRateThreshold == 0 {
		policy.FailureRateThreshold = 50
	}

	if policy.SlowCallRateThreshold == 0 {
		policy.SlowCallRateThreshold = 100
	}

	if url.policy.SlidingWindowType == "TIME_BASED" {
		policy.SlidingWindowType = circuitbreaker.TimeBased
	}

	if url.policy.SlidingWindowSize == 0 {
		policy.SlidingWindowSize = 100
	}

	if policy.PermittedNumberOfCallsInHalfOpen == 0 {
		policy.PermittedNumberOfCallsInHalfOpen = 10
	}

	if policy.MinimumNumberOfCalls == 0 {
		policy.MinimumNumberOfCalls = 100
	}

	if policy.SlowCallDurationThreshold <= 0 {
		policy.SlowCallDurationThreshold = time.Minute
	}

	if policy.MaxWaitDurationInHalfOpen < 0 {
		policy.MaxWaitDurationInHalfOpen = 0
	}

	if policy.WaitDurationInOpen <= 0 {
		policy.WaitDurationInOpen = time.Minute
	}

	url.cb = circuitbreaker.New(&policy)
}

// NewCircuitBreaker creates a new circuit breaker from spec
func NewCircuitBreaker(spec *CircuitBreakerSpec) *CircuitBreaker {
	for _, p := range spec.Policies {
		sort.Ints(p.ExceptionalStatusCode)
	}

	for _, u := range spec.URLs {
		name := u.PolicyRef
		if name == "" {
			name = spec.DefaultPolicyRef
		}

		for _, p := range spec.Policies {
			if p.Name == name {
				u.policy = p
				break
			}
		}

		u.createCircuitBreaker()
		u.cb.SetStateListener(func(url *CircuitBreakerURLRule) func(oldState, newState circuitbreaker.State) {
			return func(oldState, newState circuitbreaker.State) {
				// TODO: log state change event
			}
		}(u))
	}

	return &CircuitBreaker{spec: spec}
}

// Handle handles HTTP request
func (cb *CircuitBreaker) Handle(ctx context.HTTPContext) string {
	for _, u := range cb.spec.URLs {
		if !u.Match(ctx.Request()) {
			continue
		}

		permitted, stateID := u.cb.AcquirePermission()
		if !permitted {
			return "circuitBreaker"
		}

		ctx.AddHandlerWrapper("circuitBreaker", func(fn context.HandlerFunc) context.HandlerFunc {
			return func() string {
				start := time.Now()

				defer func() {
					if e := recover(); e != nil {
						d := time.Now().Sub(start)
						err, ok := e.(error)
						if !ok {
							err = fmt.Errorf("unknown error: %v", e)
						}
						u.cb.RecordResult(stateID, err, d)
						panic(e)
					}
				}()

				var err error
				result := fn()
				duration := time.Now().Sub(start)
				if result != "" {
					err = fmt.Errorf("result is: %s", result)
				} else {
					code := ctx.Response().StatusCode()
					idx := sort.SearchInts(u.policy.ExceptionalStatusCode, code)
					if idx < len(u.policy.ExceptionalStatusCode) && u.policy.ExceptionalStatusCode[idx] == code {
						err = fmt.Errorf("status code is: %d", code)
					}
				}

				u.cb.RecordResult(stateID, err, duration)
				return result
			}
		})
		break
	}
	return ""
}
