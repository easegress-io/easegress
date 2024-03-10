/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package resilience

import (
	"context"
	"errors"
	"strings"
	"time"

	libcb "github.com/megaease/easegress/v2/pkg/util/circuitbreaker"
)

// CircuitBreakerKind is the kind of CircuitBreaker.
var CircuitBreakerKind = &Kind{
	Name: "CircuitBreaker",

	DefaultPolicy: func() Policy {
		return &CircuitBreakerPolicy{
			CircuitBreakerRule: CircuitBreakerRule{
				FailureRateThreshold:             50,
				SlowCallRateThreshold:            100,
				SlidingWindowType:                "COUNT_BASED",
				SlidingWindowSize:                100,
				PermittedNumberOfCallsInHalfOpen: 10,
				MinimumNumberOfCalls:             100,
				SlowCallDurationThreshold:        "1m",
				WaitDurationInOpen:               "1m",
			},
		}
	},
}

var _ Policy = (*CircuitBreakerPolicy)(nil)

// ErrShortCircuited is the error returned by a circuit breaker when the
// circuit was short circuited.
var ErrShortCircuited = errors.New("the call was short circuited")

type (
	// CircuitBreakerPolicy defines the circuit break policy.
	CircuitBreakerPolicy struct {
		BaseSpec           `json:",inline"`
		CircuitBreakerRule `json:",inline"`
	}

	// CircuitBreakerRule is the detailed config of circuit breaker.
	CircuitBreakerRule struct {
		SlidingWindowType                string `json:"slidingWindowType,omitempty"  jsonschema:"enum=COUNT_BASED,enum=TIME_BASED"`
		FailureRateThreshold             uint8  `json:"failureRateThreshold,omitempty" jsonschema:"minimum=1,maximum=100"`
		SlowCallRateThreshold            uint8  `json:"slowCallRateThreshold,omitempty" jsonschema:"minimum=1,maximum=100"`
		SlidingWindowSize                uint32 `json:"slidingWindowSize,omitempty" jsonschema:"minimum=1"`
		PermittedNumberOfCallsInHalfOpen uint32 `json:"permittedNumberOfCallsInHalfOpenState,omitempty"`
		MinimumNumberOfCalls             uint32 `json:"minimumNumberOfCalls,omitempty"`
		SlowCallDurationThreshold        string `json:"slowCallDurationThreshold,omitempty" jsonschema:"format=duration"`
		MaxWaitDurationInHalfOpen        string `json:"maxWaitDurationInHalfOpenState,omitempty" jsonschema:"format=duration"`
		WaitDurationInOpen               string `json:"waitDurationInOpenState,omitempty" jsonschema:"format=duration"`
	}
)

// Validate validates the CircuitBreakPolicy.
func (p *CircuitBreakerPolicy) Validate() error {
	// TODO
	return nil
}

// CreateWrapper creates a Wrapper.
func (p *CircuitBreakerPolicy) CreateWrapper() Wrapper {
	policy := &libcb.Policy{
		FailureRateThreshold:             p.FailureRateThreshold,
		SlowCallRateThreshold:            p.SlowCallRateThreshold,
		SlidingWindowType:                libcb.CountBased,
		SlidingWindowSize:                p.SlidingWindowSize,
		PermittedNumberOfCallsInHalfOpen: p.PermittedNumberOfCallsInHalfOpen,
		MinimumNumberOfCalls:             p.MinimumNumberOfCalls,
	}

	if strings.ToUpper(p.SlidingWindowType) == "TIME_BASED" {
		policy.SlidingWindowType = libcb.TimeBased
	}

	if d := p.SlowCallDurationThreshold; d != "" {
		policy.SlowCallDurationThreshold, _ = time.ParseDuration(d)
	} else {
		policy.SlowCallDurationThreshold = time.Minute
	}

	if d := p.MaxWaitDurationInHalfOpen; d != "" {
		policy.MaxWaitDurationInHalfOpen, _ = time.ParseDuration(d)
	}

	if d := p.WaitDurationInOpen; d != "" {
		policy.WaitDurationInOpen, _ = time.ParseDuration(d)
	} else {
		policy.WaitDurationInOpen = time.Minute
	}

	return circuitBreakerWrapper{CircuitBreaker: libcb.New(policy)}
}

type circuitBreakerWrapper struct {
	*libcb.CircuitBreaker
}

// Wrap wraps the handler function.
func (w circuitBreakerWrapper) Wrap(handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context) error {
		var err error

		permitted, stateID := w.AcquirePermission()
		if !permitted {
			return ErrShortCircuited
		}

		start := time.Now()

		panicked := true
		defer func() {
			if panicked {
				w.RecordResult(stateID, true, time.Since(start))
			}
		}()

		err = handler(ctx)
		w.RecordResult(stateID, err != nil, time.Since(start))

		panicked = false
		return err
	}
}
