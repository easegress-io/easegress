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

package circuitbreaker

import (
	"fmt"
	"os"
	"testing"
	"time"
)

var now time.Time

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
			cb.RecordResult(stateID, false, time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}
	// insert 10 failure results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, true, time.Millisecond)
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
			cb.RecordResult(stateID, false, time.Millisecond)
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
			cb.RecordResult(stateID, false, time.Millisecond)
		} else {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		}
	}
	// insert 12 slow results
	for i := 0; i < 12; i++ {
		if permitted, stateID := cb.AcquirePermission(); permitted {
			cb.RecordResult(stateID, false, 11*time.Millisecond)
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
			cb.RecordResult(stateID, false, 11*time.Millisecond)
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
	policy := NewPolicy(50, 60, CountBased, 20, 5, 10,
		10*time.Millisecond, 5*time.Second, 5*time.Second)

	cb := New(policy)
	cb.window.Reset()
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
			cb.RecordResult(stateID, false, time.Millisecond)
		}
	}
	// insert 10 failure results
	for i := 0; i < 10; i++ {
		if permitted, stateID := cb.AcquirePermission(); !permitted {
			t.Errorf("acquire permission should succeeded, i = %d", i)
		} else if cb.State() != StateClosed {
			t.Errorf("circuit breaker state should be Closed")
		} else {
			cb.RecordResult(stateID, true, time.Millisecond)
		}
	}
	// state should transit to open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
}

func TestTimeBased(t *testing.T) {
	policy := NewPolicy(50, 60, TimeBased, 20, 5, 10,
		10*time.Millisecond, 5*time.Second, 5*time.Second)

	cb := New(policy)
	cb.SetStateListener(func(event *Event) {
		fmt.Printf("%v\n", event)
	})
	cb.window.Reset()
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
			cb.RecordResult(stateID, false, time.Millisecond)
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
			cb.RecordResult(stateID, true, time.Millisecond)
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
		cb.RecordResult(stateID, false, time.Millisecond)
	} else {
		t.Errorf("acquire permission should succeeded")
	}
	// state should be open now
	if cb.State() != StateOpen {
		t.Errorf("circuit breaker state should be Open")
	}
}
