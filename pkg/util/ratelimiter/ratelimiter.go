package ratelimiter

import (
	"sync"
	"time"
)

var (
	// for unit testing cases to mock 'time.Now' only
	nowFunc = time.Now
)

type (
	// Policy defines the policy of a rate limiter
	Policy struct {
		TimeoutDuration    time.Duration
		LimitRefreshPeriod time.Duration
		LimitForPeriod     int64
	}

	// StateListenerFunc is a listener function to listen state transit event
	StateListenerFunc func()

	// RateLimiter defines a rate limiter
	RateLimiter struct {
		lock        sync.Mutex
		policy      *Policy
		startTime   time.Time
		activeCycle int64

		activePermissions      int64
		accumulatedPermissions int64
		listener               StateListenerFunc
	}
)

// NewPolicy create and initialize a policy with default configuration
func NewPolicy() *Policy {
	return &Policy{
		TimeoutDuration:    5 * time.Second,
		LimitRefreshPeriod: 500 * time.Nanosecond,
		LimitForPeriod:     50,
	}
}

// New creates a rate limiter based on `policy`,
func New(policy *Policy) *RateLimiter {
	rl := &RateLimiter{
		policy:    policy,
		startTime: nowFunc(),
	}
	return rl
}

// SetStateListener sets a state listener for the RateLimiter
func (rl *RateLimiter) SetStateListener(listener StateListenerFunc) {
	rl.lock.Lock()
	defer rl.lock.Unlock()
	rl.listener = listener
}

// AccquirePermission waits a permission from the rate limiter
// returns true if the request is permitted and false if timed out
func (rl *RateLimiter) AccquirePermission() bool {
	cycle := int64(nowFunc().Sub(rl.startTime) / rl.policy.LimitRefreshPeriod)

	if cycle > rl.activeCycle {
	}

	return false
}
