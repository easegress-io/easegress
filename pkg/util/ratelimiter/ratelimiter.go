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

// Package ratelimiter provides a rate limiter
package ratelimiter

import (
	"sync"
	"time"
)

// for unit testing cases to mock 'time.Now' only
var nowFunc = time.Now

type (
	// State is rate limiter state
	State uint8

	// Policy defines the policy of a rate limiter
	Policy struct {
		TimeoutDuration    time.Duration
		LimitRefreshPeriod time.Duration
		LimitForPeriod     int
	}

	// Event defines the event of rate limiter
	Event struct {
		Time  time.Time
		State string
	}

	// EventListenerFunc is a listener function to listen state transit event
	EventListenerFunc func(event *Event)

	// RateLimiter defines a rate limiter
	RateLimiter struct {
		lock      sync.Mutex
		state     State
		policy    *Policy
		startTime time.Time
		cycle     int
		tokens    int
		listener  EventListenerFunc
	}
)

// circuit breaker states
const (
	StateNormal = iota
	StateLimiting
	StateDisabled
)

var stateStrings = []string{
	"Normal",
	"Limiting",
	"Disabled",
}

// NewPolicy create and initialize a policy
func NewPolicy(timeout, refresh time.Duration, limit int) *Policy {
	return &Policy{
		TimeoutDuration:    timeout,
		LimitRefreshPeriod: refresh,
		LimitForPeriod:     limit,
	}
}

// NewDefaultPolicy create and initialize a policy with default configuration
func NewDefaultPolicy() *Policy {
	return NewPolicy(100*time.Millisecond, 10*time.Millisecond, 50)
}

// New creates a rate limiter based on `policy`,
func New(policy *Policy) *RateLimiter {
	rl := &RateLimiter{
		policy:    policy,
		startTime: nowFunc(),
	}
	return rl
}

// SetState sets the state of the rate limiter to `state`
func (rl *RateLimiter) SetState(state State) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	if rl.state == state {
		return
	}
	if rl.state == StateDisabled {
		rl.cycle = 0
		rl.tokens = 0
		rl.startTime = nowFunc()
	}
	rl.state = state
}

// SetStateListener sets a state listener for the RateLimiter
func (rl *RateLimiter) SetStateListener(listener EventListenerFunc) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	rl.listener = listener
}

func (rl *RateLimiter) notifyListener(tm time.Time, state State) {
	if rl.listener != nil {
		event := Event{
			Time:  tm,
			State: stateStrings[state],
		}
		go rl.listener(&event)
	}
}

func (rl *RateLimiter) acquirePermission(count int) (bool, time.Duration) {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	if rl.state == StateDisabled {
		return true, 0
	}

	now := nowFunc()

	// max tokens could be permitted(including reserved) from the beginning of current cycle
	maxTokens := rl.policy.LimitForPeriod
	maxTokens *= int(rl.policy.TimeoutDuration/rl.policy.LimitRefreshPeriod) + 1

	// current cycle index
	cycle := int(now.Sub(rl.startTime) / rl.policy.LimitRefreshPeriod)

	// rl.tokens is the number of permitted tokens from the beginning of rl.cycle
	// tokens is the number of tokens have already been permitted from the beginning
	// of current cycle. note tokens could be less than zero
	tokens := rl.tokens - (cycle-rl.cycle)*rl.policy.LimitForPeriod
	if tokens < 0 {
		tokens = 0
	}

	// reject if already reached the permission limitation
	if tokens >= maxTokens {
		return false, rl.policy.TimeoutDuration
	}

	// permit another token
	rl.tokens = tokens + count
	rl.cycle = cycle

	// if there are still free tokens in current cycle
	if tokens < rl.policy.LimitForPeriod {
		if rl.state != StateNormal {
			rl.state = StateNormal
			rl.notifyListener(now, rl.state)
		}
		return true, 0
	}

	// no free tokens in current cycle, we can permit the request, but we need also
	// return a duration which the caller must wait before proceed.
	if rl.state != StateLimiting {
		rl.state = StateLimiting
		rl.notifyListener(now, rl.state)
	}

	var timeToWait time.Duration
	cycle += tokens / rl.policy.LimitForPeriod
	d := rl.policy.LimitRefreshPeriod * time.Duration(cycle)
	timeToWait = rl.startTime.Add(d).Sub(now)

	return true, timeToWait
}

// AcquirePermission acquires a permission from the rate limiter.
// returns true if the request is permitted and false otherwise.
// when permitted, the caller should wait returned duration before action.
func (rl *RateLimiter) AcquirePermission() (bool, time.Duration) {
	return rl.acquirePermission(1)
}

// AcquireNPermission acquires N permission from the rate limiter.
// returns true if the request is permitted and false otherwise.
// when permitted, the caller should wait returned duration before action.
func (rl *RateLimiter) AcquireNPermission(n int) (bool, time.Duration) {
	return rl.acquirePermission(n)
}

// WaitPermission waits a permission from the rate limiter
// returns true if the request is permitted and false if timed out
func (rl *RateLimiter) WaitPermission() bool {
	permitted, d := rl.AcquirePermission()
	if d > 0 {
		time.Sleep(d)
	}

	return permitted
}
