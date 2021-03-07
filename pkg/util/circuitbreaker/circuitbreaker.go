package circuitbreaker

import (
	"sync"
	"time"
)

var (
	// for unit testing cases to mock 'time.Now' only
	nowFunc = time.Now
)

// sliding window types
const (
	CountBased = iota
	TimeBased
)

// result types
const (
	unknown uint8 = iota
	success
	slow
	failure
)

type (
	// Window defines the interface of a window
	Window interface {
		Total() uint32
		Reset()
		PushResult(result uint8)
		FailureRate() uint8
		SlowRate() uint8
	}

	// CountBasedWindow defines the count based window
	CountBasedWindow struct {
		total     uint32
		slow      uint32
		failure   uint32
		bucketIdx int
		bucket    []uint8
	}

	timeBasedWindowBucket struct {
		total   uint32
		slow    uint32
		failure uint32
	}

	// TimeBasedWindow defines the time based window
	TimeBasedWindow struct {
		total       uint32
		slow        uint32
		failure     uint32
		beginAt     time.Time
		firstBucket int
		bucket      []timeBasedWindowBucket
	}
)

// NewCountBasedWindow creates a new count based window with `size` buckets
func NewCountBasedWindow(size uint32) *CountBasedWindow {
	cbw := &CountBasedWindow{
		bucket: make([]uint8, size),
	}
	return cbw
}

// Reset resets the count based window to initial state
func (cbw *CountBasedWindow) Reset() {
	size := len(cbw.bucket)
	*cbw = CountBasedWindow{
		bucket: make([]uint8, size),
	}
}

// Total returns the total number of results currently recorded
func (cbw *CountBasedWindow) Total() uint32 {
	return cbw.total
}

// PushResult pushes a new result into the window and may evict existing
// results if needed
func (cbw *CountBasedWindow) PushResult(result uint8) {
	// evict existing bucket, note bucket default value is 'unknown',
	// so evict does not happen when there are free buckets.
	switch cbw.bucket[cbw.bucketIdx] {
	case success:
		cbw.total--
	case slow:
		cbw.slow--
		cbw.total--
	case failure:
		cbw.failure--
		cbw.total--
	}

	cbw.total++
	switch result {
	case slow:
		cbw.slow++
	case failure:
		cbw.failure++
	}

	cbw.bucket[cbw.bucketIdx] = result
	cbw.bucketIdx++
	if cbw.bucketIdx >= len(cbw.bucket) {
		cbw.bucketIdx = 0
	}
}

// FailureRate returns the failure rate of recorded results
func (cbw *CountBasedWindow) FailureRate() uint8 {
	return uint8(cbw.failure * 100 / cbw.total)
}

// SlowRate returns the slow rate of recorded results
func (cbw *CountBasedWindow) SlowRate() uint8 {
	return uint8(cbw.slow * 100 / cbw.total)
}

// NewTimeBasedWindow creates a new time based window with `size` buckets
func NewTimeBasedWindow(size uint32) *TimeBasedWindow {
	tbw := &TimeBasedWindow{
		bucket:  make([]timeBasedWindowBucket, size),
		beginAt: nowFunc().Truncate(time.Second),
	}
	return tbw
}

// Reset resets the time based window to initial state
func (tbw *TimeBasedWindow) Reset() {
	size := len(tbw.bucket)
	*tbw = TimeBasedWindow{
		bucket:  make([]timeBasedWindowBucket, size),
		beginAt: nowFunc().Truncate(time.Second),
	}
}

// Total returns the total number of results currently recorded
func (tbw *TimeBasedWindow) Total() uint32 {
	return tbw.total
}

func (tbw *TimeBasedWindow) evict(now time.Time) {
	// check how many seconds has passed since the beginning of the window
	seconds := int(now.Sub(tbw.beginAt) / time.Second)

	// no bucket need to be evicted if seconds is less than the window size
	if seconds < len(tbw.bucket) {
		return
	}

	// evicts is how many buckets need to be evicted
	evicts := seconds - len(tbw.bucket) + 1

	// the begin time of the window need to be adjusted according to evicts
	tbw.beginAt = tbw.beginAt.Add(time.Duration(evicts) * time.Second)

	// evicts may be very large, but at most len(tbw.bucket) buckets need to
	// be evicted
	if evicts > len(tbw.bucket) {
		evicts = len(tbw.bucket)
	}

	// evict all of them
	for i := 0; i < evicts; i++ {
		// get the bucket
		b := &tbw.bucket[tbw.firstBucket]

		// deduct from window
		tbw.total -= b.total
		tbw.slow -= b.slow
		tbw.failure -= b.failure

		// reset bucket to zero
		*b = timeBasedWindowBucket{}

		// adjust the index of first bucket
		tbw.firstBucket = (tbw.firstBucket + 1) % len(tbw.bucket)
	}
}

// PushResult pushes a new result into the window and may evict existing
// results if needed
func (tbw *TimeBasedWindow) PushResult(result uint8) {
	now := nowFunc()

	tbw.evict(now)

	idx := tbw.firstBucket
	idx += int(now.Sub(tbw.beginAt) / time.Second)
	idx %= len(tbw.bucket)
	bucket := &tbw.bucket[idx]

	tbw.total++
	bucket.total++
	switch result {
	case slow:
		tbw.slow++
		bucket.slow++

	case failure:
		tbw.failure++
		bucket.failure++
	}
}

// FailureRate returns the failure rate of recorded results
func (tbw *TimeBasedWindow) FailureRate() uint8 {
	return uint8(tbw.failure * 100 / tbw.total)
}

// SlowRate returns the slow rate of recorded results
func (tbw *TimeBasedWindow) SlowRate() uint8 {
	return uint8(tbw.slow * 100 / tbw.total)
}

// circuit breaker states
const (
	Disabled = iota
	Closed
	HalfOpen
	Open
	ForceOpen
)

type (
	// Policy defines the policy of a circuit breaker
	Policy struct {
		FailureRateThreshold                  uint8         `yaml:"failureRateThreshold"`
		SlowCallRateThreshold                 uint8         `yaml:"slowCallRateThreshold"`
		SlidingWindowType                     uint8         `yaml:"slidingWindowType"`
		SlidingWindowSize                     uint32        `yaml:"slidingWindowSize"`
		PermittedNumberOfCallsInHalfOpenState uint32        `yaml:"permittedNumberOfCallsInHalfOpenState"`
		MinimumNumberOfCalls                  uint32        `yaml:"minimumNumberOfCalls"`
		SlowCallDurationThreshold             time.Duration `yaml:"slowCallDurationThreshold"`
		MaxWaitDurationInHalfOpenState        time.Duration `yaml:"maxWaitDurationInHalfOpenState"`
		WaitDurationInOpenState               time.Duration `yaml:"waitDurationInOpenState"`
	}

	// CircuitBreaker defines a circuit breaker
	CircuitBreaker struct {
		lock                    sync.Mutex
		policy                  *Policy
		state                   uint8
		transitTime             time.Time
		window                  Window
		numberOfCallsInHalfOpen uint32
	}
)

// NewPolicy create and initialize a policy with default configuration
func NewPolicy() *Policy {
	return &Policy{
		FailureRateThreshold:                  50,
		SlowCallRateThreshold:                 100,
		SlidingWindowType:                     CountBased,
		SlidingWindowSize:                     100,
		PermittedNumberOfCallsInHalfOpenState: 10,
		MinimumNumberOfCalls:                  100,
		SlowCallDurationThreshold:             time.Minute,
		MaxWaitDurationInHalfOpenState:        0,
		WaitDurationInOpenState:               time.Minute,
	}
}

// New creates a circuit breaker based on `policy`,
func New(policy *Policy) *CircuitBreaker {
	cb := &CircuitBreaker{policy: policy}
	cb.transitToClosed()
	return cb
}

// SetState sets the state of the circuit breaker to `state`
func (cb *CircuitBreaker) SetState(state uint8) {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	if state == cb.state {
		return
	}

	switch state {
	case Disabled, ForceOpen:
		cb.state = state
		cb.transitTime = nowFunc()
	case Open:
		cb.transitToOpen()
	case Closed:
		cb.transitToClosed()
	case HalfOpen:
		cb.transitToHalfOpen()
	default:
		panic("unknown target state")
	}
}

// State returns the state of the circuit breaker
func (cb *CircuitBreaker) State() uint8 {
	return cb.state
}

func (cb *CircuitBreaker) transitToClosed() {
	cb.state = Closed
	// recreate the window to remove all existing results to avoid jitter
	if cb.policy.SlidingWindowType == CountBased {
		cb.window = NewCountBasedWindow(cb.policy.SlidingWindowSize)
	} else {
		cb.window = NewTimeBasedWindow(cb.policy.SlidingWindowSize)
	}
	cb.transitTime = nowFunc()
}

func (cb *CircuitBreaker) transitToHalfOpen() {
	cb.state = HalfOpen
	// always use count based window in half open state to avoid results being evicted
	cb.window = NewCountBasedWindow(cb.policy.PermittedNumberOfCallsInHalfOpenState)
	cb.numberOfCallsInHalfOpen = 0
	cb.transitTime = nowFunc()
}

func (cb *CircuitBreaker) transitToOpen() {
	cb.state = Open
	cb.transitTime = nowFunc()
}

// AcquirePermission acquires a permission from the circuit breaker,
// return true if approved and false if rejected
func (cb *CircuitBreaker) AcquirePermission() bool {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	// always return true when disabled
	if cb.state == Disabled {
		return true
	}

	// always return false when force open
	if cb.state == ForceOpen {
		return false
	}

	// always return true when closed.
	// for time based window, failure rate or slow rate may change as time elapse,
	// that's even no new result were recorded, state may transit from closed to
	// open if may sucess results are evicted by time. but we just rely on the
	// state here and leave state transition to RecordResult to keep code simple.
	if cb.state == Closed {
		return true
	}

	// when state is open, return false if open duration is less than
	// WaitDurationInOpenState. transit to half open otherwise
	if cb.state == Open {
		if nowFunc().Sub(cb.transitTime) < cb.policy.WaitDurationInOpenState {
			return false
		}
		// after WaitDurationInOpenState, transit to half open
		cb.transitToHalfOpen()
	}

	// if state is still half open after MaxWaitDurationInHalfOpenState,
	// transit back to open
	if cb.policy.MaxWaitDurationInHalfOpenState <= 0 &&
		nowFunc().Sub(cb.transitTime) > cb.policy.MaxWaitDurationInHalfOpenState {
		cb.transitToOpen()
		return false
	}

	// circuit breaker is in half open state
	if cb.numberOfCallsInHalfOpen < cb.policy.PermittedNumberOfCallsInHalfOpenState {
		cb.numberOfCallsInHalfOpen++
		return true
	}

	return false
}

// RecordResult records the result in window
func (cb *CircuitBreaker) RecordResult(err error, d time.Duration) {
	// calculate call result
	result := success
	if err != nil {
		result = failure
	} else if d >= cb.policy.SlowCallDurationThreshold {
		result = slow
	}

	cb.lock.Lock()
	defer cb.lock.Unlock()

	// don't record result in these states
	if cb.state == Disabled || cb.state == Open || cb.state == ForceOpen {
		return
	}

	cb.window.PushResult(result)

	// check if enough results were collected
	minNumOfCalls := cb.policy.MinimumNumberOfCalls
	if cb.state == HalfOpen {
		minNumOfCalls = cb.policy.PermittedNumberOfCallsInHalfOpenState
	}
	if cb.window.Total() < minNumOfCalls {
		return
	}

	// for count based window, state doesn't transit if result is success
	// but for time based window, state may transit to open even if result
	// is success as existing success results may be evicted by time.
	// note half open state always use a count based window.
	if cb.window.FailureRate() >= cb.policy.FailureRateThreshold {
		cb.transitToOpen()
	} else if cb.window.SlowRate() >= cb.policy.SlowCallRateThreshold {
		cb.transitToOpen()
	} else if cb.state == HalfOpen {
		cb.transitToClosed()
	}
}
