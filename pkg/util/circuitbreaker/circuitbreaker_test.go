package circuitbreaker

import (
	"errors"
	"os"
	"testing"
	"time"
)

var (
	now time.Time
	err = errors.New("some error")
)

func setup() {
	now = time.Now()
	nowFunc = func() time.Time {
		return now
	}
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func runSharedCases(t *testing.T, cb *CircuitBreaker) {
	// insert 10 success results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, nil, time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}
	// insert 10 failure results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, err, time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}

	// state should transit to open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
	if permitted, _ := cb.AcquirePermission(); permitted {
		t.Errorf("acquire permission should fail")
	}

	// wait 5 seconds for state to transit to half open
	now = now.Add(5 * time.Second)

	// insert 5 success results
	for i := 0; i < 5; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateHalfOpen {
			t.Errorf("circuit breaker state should be HalfOpen")
		} else {
			cb.RecordResult(stateID, nil, time.Millisecond)
		}
	}

	// state should transit to closed now
	if cb.State() != StateClosed {
		t.Errorf("circuit breaker state should be Closed")
	}
	if permitted, _ := cb.AcquirePermission(); !permitted {
		t.Errorf("acquire permission should succeeded")
	}

	// insert 8 success results
	for i := 0; i < 8; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, nil, time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}
	// insert 12 slow results
	for i := 0; i < 12; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, nil, 11*time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}

	// state should transit to open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
	if permitted, _ := cb.AcquirePermission(); permitted {
		t.Errorf("acquire permission should fail")
	}

	// wait 5 seconds for state to transit to half open
	now = now.Add(5 * time.Second)

	// insert 5 slow results
	for i := 0; i < 5; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateHalfOpen {
			t.Errorf("circuit breaker state should be HalfOpen")
		} else {
			cb.RecordResult(stateID, nil, 11*time.Millisecond)
		}
	}

	// state should transit to open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
	if permitted, _ := cb.AcquirePermission(); permitted {
		t.Errorf("acquire permission should fail")
	}
}

func TestCountBased(t *testing.T) {
	policy := Policy{
		FailureRateThreshold:                  50,
		SlowCallRateThreshold:                 60,
		SlidingWindowType:                     CountBased,
		SlidingWindowSize:                     20,
		PermittedNumberOfCallsInHalfOpenState: 5,
		MinimumNumberOfCalls:                  10,
		SlowCallDurationThreshold:             time.Millisecond * 10,
		MaxWaitDurationInHalfOpenState:        5 * time.Second,
		WaitDurationInOpenState:               5 * time.Second,
	}

	cb := New(&policy)
	runSharedCases(t, cb)

	// transit to closed
	cb.SetState(StateClosed)
	// insert 12 success results
	for i := 0; i < 12; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateClosed {
			t.Errorf("circuit breaker state should be Closed")
		} else {
			cb.RecordResult(stateID, nil, time.Millisecond)
		}
	}
	// insert 10 failure results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateClosed {
			t.Errorf("circuit breaker state should be Closed")
		} else {
			cb.RecordResult(stateID, err, time.Millisecond)
		}
	}
	// state should transit to open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
}

func TestTimeBased(t *testing.T) {
	policy := Policy{
		FailureRateThreshold:                  50,
		SlowCallRateThreshold:                 60,
		SlidingWindowType:                     TimeBased,
		SlidingWindowSize:                     20,
		PermittedNumberOfCallsInHalfOpenState: 5,
		MinimumNumberOfCalls:                  10,
		SlowCallDurationThreshold:             time.Millisecond * 10,
		MaxWaitDurationInHalfOpenState:        5 * time.Second,
		WaitDurationInOpenState:               5 * time.Second,
	}

	cb := New(&policy)
	runSharedCases(t, cb)

	// transit to closed
	cb.SetState(StateClosed)
	// insert 12 success results
	for i := 0; i < 12; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateClosed {
			t.Errorf("circuit breaker state should be Closed")
		} else {
			cb.RecordResult(stateID, nil, time.Millisecond)
		}
		now = now.Add(500 * time.Millisecond)
	}
	// insert 10 failure results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateClosed {
			t.Errorf("circuit breaker state should be Closed")
		} else {
			cb.RecordResult(stateID, err, time.Millisecond)
		}
	}
	// state should be closed
	if cb.State() != StateClosed {
		t.Errorf("circuit breaker state should be Closed")
	}
	// evicts some success results
	now = now.Add(15500 * time.Millisecond)
	// add a new success result
	if permitted, stateID := cb.AcquirePermission(); permitted {
		cb.RecordResult(stateID, nil, time.Millisecond)
	} else {
		t.Errorf("acquire permission should succeeded")
	}
	// state should be open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
}
