/*
 * Copyright (c) 2017, MegaEase
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

	libcb "github.com/megaease/easegress/pkg/util/circuitbreaker"
)

var circuitBreakKind = &Kind{
	Name: "CircuitBreaker",

	DefaultPolicy: func() Policy {
		return &CircuitBreakPolicy{
			FailureRateThreshold:             50,
			SlowCallRateThreshold:            100,
			SlidingWindowType:                "COUNT_BASED",
			SlidingWindowSize:                100,
			PermittedNumberOfCallsInHalfOpen: 10,
			MinimumNumberOfCalls:             100,
			SlowCallDurationThreshold:        "1m",
			WaitDurationInOpen:               "1m",
		}
	},
}

var _ Policy = (*CircuitBreakPolicy)(nil)

// ErrShortCircuited is the error returned by a circuit breaker when the
// circuit was short circuited.
var ErrShortCircuited = errors.New("the call was short circuited")

// CircuitBreakPolicy defines the circuit break policy.
type CircuitBreakPolicy struct {
	BaseSpec                         `yaml:",inline"`
	SlidingWindowType                string `yaml:"slidingWindowType"  jsonschema:"omitempty,enum=COUNT_BASED,enum=TIME_BASED"`
	FailureRateThreshold             uint8  `yaml:"failureRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
	SlowCallRateThreshold            uint8  `yaml:"slowCallRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
	CountingNetworkError             bool   `yaml:"countingNetworkError" jsonschema:"omitempty"`
	SlidingWindowSize                uint32 `yaml:"slidingWindowSize" jsonschema:"omitempty,minimum=1"`
	PermittedNumberOfCallsInHalfOpen uint32 `yaml:"permittedNumberOfCallsInHalfOpenState" jsonschema:"omitempty"`
	MinimumNumberOfCalls             uint32 `yaml:"minimumNumberOfCalls" jsonschema:"omitempty"`
	SlowCallDurationThreshold        string `yaml:"slowCallDurationThreshold" jsonschema:"omitempty,format=duration"`
	MaxWaitDurationInHalfOpen        string `yaml:"maxWaitDurationInHalfOpenState" jsonschema:"omitempty,format=duration"`
	WaitDurationInOpen               string `yaml:"waitDurationInOpenState" jsonschema:"omitempty,format=duration"`
}

// Validate validates the CircuitBreakPolicy.
func (p *CircuitBreakPolicy) Validate() error {
	// TODO
	return nil
}

// CreateWrapper creates a Wrapper.
func (p *CircuitBreakPolicy) CreateWrapper() Wrapper {
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

	return circuitBreakWrapper{CircuitBreaker: libcb.New(policy)}
}

type circuitBreakWrapper struct {
	*libcb.CircuitBreaker
}

// Wrap wraps the handler function.
func (w circuitBreakWrapper) Wrap(handler HandlerFunc) HandlerFunc {
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
