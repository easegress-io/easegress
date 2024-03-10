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

package ratelimiter

import (
	"fmt"
	"sync"
	"time"
)

type (
	// MultiPolicy defines the policy of a rate limiter
	MultiPolicy struct {
		TimeoutDuration    time.Duration
		LimitRefreshPeriod time.Duration
		LimitForPeriod     []int
	}

	// MultiRateLimiter defines a rate limiter
	MultiRateLimiter struct {
		lock      sync.Mutex
		state     State
		policy    *MultiPolicy
		startTime time.Time
		cycle     int
		tokens    []int
	}
)

// NewMultiPolicy create and initialize a policy
func NewMultiPolicy(timeout, refresh time.Duration, limit []int) *MultiPolicy {
	return &MultiPolicy{
		TimeoutDuration:    timeout,
		LimitRefreshPeriod: refresh,
		LimitForPeriod:     limit,
	}
}

// NewMulti creates a multi rate limiter based on `policy`,
func NewMulti(policy *MultiPolicy) *MultiRateLimiter {
	rl := &MultiRateLimiter{
		policy:    policy,
		startTime: nowFunc(),
		tokens:    make([]int, len(policy.LimitForPeriod)),
	}
	return rl
}

// SetState sets the state of the rate limiter to `state`
func (rl *MultiRateLimiter) SetState(state State) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	if rl.state == state {
		return
	}
	if rl.state == StateDisabled {
		rl.cycle = 0
		rl.tokens = make([]int, len(rl.policy.LimitForPeriod))
		rl.startTime = nowFunc()
	}
	rl.state = state
}

// AcquirePermission acquires a permission from the rate limiter.
// returns true if the request is permitted and false otherwise.
// when permitted, the caller should wait returned duration before action.
func (rl *MultiRateLimiter) AcquirePermission(count []int) (bool, time.Duration, error) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	if rl.state == StateDisabled {
		return true, 0, nil
	}
	if len(count) != len(rl.policy.LimitForPeriod) {
		return false, 0, fmt.Errorf("MultiRateLimiter policy contain %v tokens but get %v tokens", rl.policy.LimitForPeriod, count)
	}

	now := nowFunc()

	// max tokens could be permitted(including reserved) from the beginning of current cycle
	maxTokens := make([]int, len(rl.policy.LimitForPeriod))
	copy(maxTokens, rl.policy.LimitForPeriod)
	scale := int(rl.policy.TimeoutDuration/rl.policy.LimitRefreshPeriod) + 1
	for i := range maxTokens {
		maxTokens[i] *= scale
	}

	// current cycle index
	cycle := int(now.Sub(rl.startTime) / rl.policy.LimitRefreshPeriod)

	// rl.tokens is the number of permitted tokens from the beginning of rl.cycle
	// tokens is the number of tokens have already been permitted from the beginning
	// of current cycle. note tokens could be less than zero
	tokens := make([]int, len(rl.tokens))
	for i, token := range rl.tokens {
		newToken := token - (cycle-rl.cycle)*rl.policy.LimitForPeriod[i]
		if newToken < 0 {
			newToken = 0
		}
		tokens[i] = newToken
	}

	// reject if already reached the permission limitation
	for i, token := range tokens {
		if token >= maxTokens[i] {
			return false, rl.policy.TimeoutDuration, nil
		}
	}

	// permit another token
	for i := range rl.tokens {
		rl.tokens[i] = tokens[i] + count[i]
	}
	rl.cycle = cycle

	// if there are still free tokens in current cycle
	allFree := true
	for i, token := range tokens {
		if token >= rl.policy.LimitForPeriod[i] {
			allFree = false
			break
		}
	}
	if allFree {
		if rl.state != StateNormal {
			rl.state = StateNormal
		}
		return true, 0, nil
	}

	// no free tokens in current cycle, we can permit the request, but we need also
	// return a duration which the caller must wait before proceed.

	var timeToWait time.Duration
	for i, token := range tokens {
		tmpCycle := cycle + token/rl.policy.LimitForPeriod[i]
		d := rl.policy.LimitRefreshPeriod * time.Duration(tmpCycle)
		wait := rl.startTime.Add(d).Sub(now)
		if wait > timeToWait {
			timeToWait = wait
		}
	}
	return true, timeToWait, nil
}

// WaitPermission waits a permission from the rate limiter
// returns true if the request is permitted and false if timed out
func (rl *MultiRateLimiter) WaitPermission(counts []int) (bool, error) {
	permitted, d, err := rl.AcquirePermission(counts)
	if err != nil {
		return false, err
	}
	if d > 0 {
		time.Sleep(d)
	}
	return permitted, nil
}
