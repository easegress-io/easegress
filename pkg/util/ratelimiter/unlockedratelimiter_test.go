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
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestUnlockedConcurrent(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	var wg sync.WaitGroup
	rl := NewUnlocked(policy)

	waitPermission := func() bool {
		rl.Lock()
		permitted, d := rl.CheckPermission(1)
		if permitted {
			d = rl.DoLastCheck()
			rl.Unlock()
			time.Sleep(d)
		} else {
			rl.Unlock()
			time.Sleep(d)
		}
		return permitted
	}

	fn := func() {
		permitted := waitPermission()
		if !permitted {
			t.Errorf("WaitPermission should succeed")
		}
		wg.Done()
	}

	wg.Add(30)
	for i := 0; i < 30; i++ {
		go fn()
	}
	wg.Wait()

	if waitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 5)
	if waitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 5)
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go fn()
	}
	wg.Wait()

	if waitPermission() {
		t.Errorf("WaitPermission should fail")
	}

	now = now.Add(time.Millisecond * 100)
	wg.Add(30)
	for i := 0; i < 30; i++ {
		go fn()
	}
	wg.Wait()

	if waitPermission() {
		t.Errorf("WaitPermission should fail")
	}
}
func TestUnlockedRateLimiter(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	limiter := NewUnlocked(policy)
	limiter.Lock()
	limiter.SetStateListener(func(event *Event) {
		fmt.Printf("%v\n", event)
	})
	limiter.SetState(StateNormal)
	limiter.Unlock()

	setState := func(state State) {
		limiter.Lock()
		limiter.SetState(state)
		limiter.Unlock()
	}
	acquirePermission := func() (bool, time.Duration) {
		limiter.Lock()
		defer limiter.Unlock()

		permitted, d := limiter.CheckPermission(1)
		if !permitted {
			return permitted, d
		}
		d = limiter.DoLastCheck()
		return permitted, d
	}

	for i := 0; i < 30; i++ {
		permitted, d := acquirePermission()
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	setState(StateDisabled)
	if permitted, d := acquirePermission(); !permitted {
		t.Errorf("AcquirePermission should succeeded")
	} else if d != 0 {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	setState(StateNormal)
	for i := 0; i < 30; i++ {
		permitted, d := acquirePermission()
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	setState(StateLimiting)
	if permitted, d := acquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	setState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	if permitted, d := acquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		if permitted, d := acquirePermission(); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != policy.TimeoutDuration-time.Millisecond {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	if permitted, d := acquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 30; i++ {
		if permitted, d := acquirePermission(); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != time.Duration(i/policy.LimitForPeriod)*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	if permitted, d := acquirePermission(); permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}
	setState(StateDisabled)
}

func TestUnlockedRateLimiterN(t *testing.T) {
	policy := NewPolicy(50*time.Millisecond, 10*time.Millisecond, 5)

	limiter := NewUnlocked(policy)
	limiter.Lock()
	limiter.SetStateListener(func(event *Event) {
		fmt.Printf("%v\n", event)
	})
	limiter.SetState(StateNormal)
	limiter.Unlock()

	setState := func(state State) {
		limiter.Lock()
		limiter.SetState(state)
		limiter.Unlock()
	}
	acquirePermission := func(n int) (bool, time.Duration) {
		limiter.Lock()
		defer limiter.Unlock()

		permitted, d := limiter.CheckPermission(n)
		if !permitted {
			return permitted, d
		}
		d = limiter.DoLastCheck()
		return permitted, d
	}

	for i := 0; i < 15; i++ {
		permitted, _ := acquirePermission(2)
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateDisabled)
	if permitted, _ := acquirePermission(1); !permitted {
		t.Errorf("AcquirePermission should succeeded")
	}

	limiter.SetState(StateNormal)
	for i := 0; i < 10; i++ {
		permitted, _ := acquirePermission(3)
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateLimiting)
	if permitted, _ := acquirePermission(1); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	limiter.SetState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	if permitted, _ := acquirePermission(1); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		if permitted, _ := acquirePermission(1); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	if permitted, _ := acquirePermission(1); permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 5; i++ {
		if permitted, _ := acquirePermission(6); !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	if permitted, _ := acquirePermission(1); permitted {
		t.Errorf("AcquirePermission should fail")
	}
	setState(StateDisabled)
}
