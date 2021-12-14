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

package ratelimiter

import (
	"sync"
	"time"
)

// UnlockedRateLimiter defines a rate limiter that expose lock method to users, so
// multiple UnlockedRateLimiters can work together. For example, we have two UnlockedRateLimiter
// called rl1 and rl2. Then we can lock rl1 and rl2 to check it we can get permission from both of
// them, and then do acquire together. Finally, unlock both rl1 and rl2.
type UnlockedRateLimiter struct {
	lock      sync.Mutex
	state     State
	policy    *Policy
	startTime time.Time
	cycle     int
	tokens    int
	listener  EventListenerFunc

	// cache
	cacheTokens int
	cacheCount  int
	cacheCycle  int
	cacheNow    time.Time
}

// New creates a rate limiter based on `policy`,
func NewUnlocked(policy *Policy) *UnlockedRateLimiter {
	rl := &UnlockedRateLimiter{
		policy:    policy,
		startTime: nowFunc(),
	}
	return rl
}

func (rl *UnlockedRateLimiter) Lock() {
	rl.lock.Lock()
}

func (rl *UnlockedRateLimiter) Unlock() {
	rl.lock.Unlock()
}

// SetState sets the state of the rate limiter to `state`
func (rl *UnlockedRateLimiter) SetState(state State) {
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
func (rl *UnlockedRateLimiter) SetStateListener(listener EventListenerFunc) {
	rl.listener = listener
}

func (rl *UnlockedRateLimiter) notifyListener(tm time.Time, state State) {
	if rl.listener != nil {
		event := Event{
			Time:  tm,
			State: stateStrings[state],
		}
		go rl.listener(&event)
	}
}

func (rl *UnlockedRateLimiter) CheckPermission(count int) (bool, time.Duration) {
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

	rl.cacheTokens = tokens
	rl.cacheCount = count
	rl.cacheNow = now
	rl.cacheCycle = cycle
	// reject if already reached the permission limitation
	if tokens >= maxTokens {
		return false, rl.policy.TimeoutDuration
	}
	return true, 0
}

func (rl *UnlockedRateLimiter) DoLastCheck() time.Duration {
	if rl.state == StateDisabled {
		return 0
	}

	tokens := rl.cacheTokens
	count := rl.cacheCount
	now := rl.cacheNow
	cycle := rl.cacheCycle

	// permit another token
	rl.tokens = tokens + count
	rl.cycle = cycle

	// if there are still free tokens in current cycle
	if tokens < rl.policy.LimitForPeriod {
		if rl.state != StateNormal {
			rl.state = StateNormal
			rl.notifyListener(now, rl.state)
		}
		return 0
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

	return timeToWait
}
