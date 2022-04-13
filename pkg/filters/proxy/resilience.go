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

package proxy

import (
	stdcontext "context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/resilience"
	libcb "github.com/megaease/easegress/pkg/util/circuitbreaker"
	"github.com/megaease/easegress/pkg/util/fasttime"
)

type Handler func(stdctx stdcontext.Context, ctx *context.Context) (result string)

type ResilienceSpec struct {
	RetryPolicy        string `yaml:"retryPolicy" jsonschema:"omitempty"`
	TimeLimitPolicy    string `yaml:"timeLimitPolicy" jsonschema:"omitempty"`
	CircuitBreakPolicy string `yaml:"circuitBreakPolicy" jsonschema:"omitempty"`
	FailureCodes       []int  `yaml:"failureCodes" jsonschema:"omitempty"`
}

const (
	randomBackOff        = "randomBackOff"
	exponentiallyBackOff = "exponentiallyBackOff"
)

type Resilience struct {
	retry        *ResilienceRetry
	timeLimit    *ResilienceTimeLimit
	circuitbreak *ResilienceCircuitBreak
}

type FailFn func(ctx *context.Context, result string) bool

func getFailureFn(failureCodes []int) FailFn {
	codeMap := make(map[int]struct{})
	for _, code := range failureCodes {
		codeMap[code] = struct{}{}
	}

	return func(ctx *context.Context, result string) bool {
		if result == "" {
			return false
		}
		resp := ctx.Response().(*httpprot.Response)
		if _, ok := codeMap[resp.StatusCode()]; ok {
			return true
		}
		return false
	}
}

func newResilience(spec *ResilienceSpec, policies map[string]resilience.Policy) (*Resilience, error) {
	res := &Resilience{}
	failFn := getFailureFn(spec.FailureCodes)

	if spec.RetryPolicy != "" {
		policy, ok := policies[spec.RetryPolicy]
		if !ok {
			return nil, fmt.Errorf("retry policy %s not found", spec.RetryPolicy)
		}
		retryPolicy, ok := policy.(*resilience.RetryPolicy)
		if !ok {
			return nil, fmt.Errorf("policy %s is not a retry policy", spec.RetryPolicy)
		}
		retry, err := newResilienceRetry(retryPolicy, failFn)
		if err != nil {
			return nil, fmt.Errorf("invalid retry policy %v", spec.RetryPolicy)
		}
		res.retry = retry
	}

	if spec.TimeLimitPolicy != "" {
		policy, ok := policies[spec.TimeLimitPolicy]
		if !ok {
			return nil, fmt.Errorf("timelimit policy %s not found", spec.TimeLimitPolicy)
		}
		timeLimitPolicy, ok := policy.(*resilience.TimeLimitPolicy)
		if !ok {
			return nil, fmt.Errorf("policy %s is not a retry policy", spec.TimeLimitPolicy)
		}
		timelimit, err := newResilienceTimeLimit(timeLimitPolicy, failFn)
		if err != nil {
			return nil, fmt.Errorf("invalid timelimit policy %v", spec.TimeLimitPolicy)
		}
		res.timeLimit = timelimit
	}

	if spec.CircuitBreakPolicy != "" {
		policy, ok := policies[spec.CircuitBreakPolicy]
		if !ok {
			return nil, fmt.Errorf("circuitbreak policy %s not found", spec.CircuitBreakPolicy)
		}
		circuitbreakPolicy, ok := policy.(*resilience.CircuitBreakPolicy)
		if !ok {
			return nil, fmt.Errorf("policy %s is not a circuitbreak policy", spec.CircuitBreakPolicy)
		}
		circuitbreak, err := newResilienceCircuitBreak(circuitbreakPolicy, failFn)
		if err != nil {
			return nil, fmt.Errorf("invalid circuitbreak policy %v", spec.CircuitBreakPolicy)
		}
		res.circuitbreak = circuitbreak
	}
	return res, nil
}

type ResilienceRetry struct {
	spec          *resilience.RetryPolicy
	waitDuration  time.Duration
	fail          FailFn
	backOffPolicy string
}

func (r *ResilienceRetry) init() {
	if r.spec.MaxAttempts <= 0 {
		r.spec.MaxAttempts = 3
	}
	if r.spec.RandomizationFactor <= 0 || r.spec.RandomizationFactor > 1 {
		r.spec.RandomizationFactor = 0
	}
	if strings.ToUpper(r.spec.BackOffPolicy) == "EXPONENTIAL" {
		r.backOffPolicy = exponentiallyBackOff
	}
	if d := r.spec.WaitDuration; d != "" {
		r.waitDuration, _ = time.ParseDuration(d)
	} else {
		r.waitDuration = time.Millisecond * 500
	}
}

func (r *ResilienceRetry) wrap(handler Handler) Handler {
	return func(stdctx stdcontext.Context, ctx *context.Context) (result string) {
		base := float64(r.waitDuration)

		for attempt := 0; attempt < r.spec.MaxAttempts; attempt++ {
			result = handler(stdctx, ctx)
			if !r.fail(ctx, result) {
				ctx.AddLazyTag(func() string {
					return fmt.Sprintf("retryer: succeeded after %d attempts", attempt)
				})
				return result
			}

			delta := base * r.spec.RandomizationFactor
			d := base - delta + float64(rand.Intn(int(delta*2+1)))

			select {
			case <-stdctx.Done():
				return result
			case <-time.After(time.Duration(d)):
			}
			if r.backOffPolicy == exponentiallyBackOff {
				base *= 1.5
			}
		}
		ctx.AddLazyTag(func() string { return fmt.Sprintf("retryer: failed after %d attempts", r.spec.MaxAttempts) })
		return result
	}
}

func newResilienceRetry(policy *resilience.RetryPolicy, failFn FailFn) (*ResilienceRetry, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	retry := &ResilienceRetry{
		spec: policy,
		fail: failFn,
	}
	retry.init()
	return retry, nil
}

type ResilienceTimeLimit struct {
	spec    *resilience.TimeLimitPolicy
	timeout time.Duration
}

func (t *ResilienceTimeLimit) init() {
	t.timeout, _ = time.ParseDuration(t.spec.TimeoutDuration)
}

func (t *ResilienceTimeLimit) wrap(handler Handler) Handler {
	return func(stdctx stdcontext.Context, ctx *context.Context) (result string) {
		timeoutCtx, cancel := stdcontext.WithTimeout(stdctx, t.timeout)
		timer := time.AfterFunc(t.timeout, func() {
			cancel()
		})

		result = handler(timeoutCtx, ctx)

		if !timer.Stop() {
			ctx.AddLazyTag(func() string { return "timelimit: timeout" })
			ctx.Response().(*httpprot.Response).SetStatusCode(http.StatusRequestTimeout)
			result = resultTimeout
		}
		return result
	}
}

func newResilienceTimeLimit(policy *resilience.TimeLimitPolicy, failFn FailFn) (*ResilienceTimeLimit, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	timelimit := &ResilienceTimeLimit{
		spec: policy,
	}
	timelimit.init()
	return timelimit, nil
}

type ResilienceCircuitBreak struct {
	spec *resilience.CircuitBreakPolicy
	fail FailFn
	cb   *libcb.CircuitBreaker
}

func (c *ResilienceCircuitBreak) init() {
	policy := &libcb.Policy{
		FailureRateThreshold:             c.spec.FailureRateThreshold,
		SlowCallRateThreshold:            c.spec.SlowCallRateThreshold,
		SlidingWindowType:                libcb.CountBased,
		SlidingWindowSize:                c.spec.SlidingWindowSize,
		PermittedNumberOfCallsInHalfOpen: c.spec.PermittedNumberOfCallsInHalfOpen,
		MinimumNumberOfCalls:             c.spec.MinimumNumberOfCalls,
	}

	if policy.FailureRateThreshold == 0 {
		policy.FailureRateThreshold = 50
	}

	if policy.SlowCallRateThreshold == 0 {
		policy.SlowCallRateThreshold = 100
	}

	if strings.ToUpper(c.spec.SlidingWindowType) == "TIME_BASED" {
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

	if d := c.spec.SlowCallDurationThreshold; d != "" {
		policy.SlowCallDurationThreshold, _ = time.ParseDuration(d)
	} else {
		policy.SlowCallDurationThreshold = time.Minute
	}

	if d := c.spec.MaxWaitDurationInHalfOpen; d != "" {
		policy.MaxWaitDurationInHalfOpen, _ = time.ParseDuration(d)
	}

	if d := c.spec.WaitDurationInOpen; d != "" {
		policy.WaitDurationInOpen, _ = time.ParseDuration(d)
	} else {
		policy.WaitDurationInOpen = time.Minute
	}
	c.cb = libcb.New(policy)
}

func (c *ResilienceCircuitBreak) wrap(handler Handler) Handler {
	return func(stdctx stdcontext.Context, ctx *context.Context) (result string) {
		permitted, stateID := c.cb.AcquirePermission()
		if !permitted {
			ctx.AddTag("circuitBreaker: circuit is broken")
			ctx.Response().(*httpprot.Response).SetStatusCode(http.StatusServiceUnavailable)
			return resultShortCircuited
		}

		start := fasttime.Now()
		defer func() {
			if e := recover(); e != nil {
				d := time.Since(start)
				c.cb.RecordResult(stateID, true, d)
				panic(e)
			}
		}()

		result = handler(stdctx, ctx)

		d := time.Since(start)
		fail := c.fail(ctx, result)
		c.cb.RecordResult(stateID, fail, d)
		return result
	}
}

func newResilienceCircuitBreak(policy *resilience.CircuitBreakPolicy, failFn FailFn) (*ResilienceCircuitBreak, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	circuitbreak := &ResilienceCircuitBreak{
		spec: policy,
		fail: failFn,
	}
	return circuitbreak, nil
}
