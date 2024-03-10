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
	"sync"
	"testing"
	"time"
)

func TestMultiConcurrent(t *testing.T) {
	policy := NewMultiPolicy(50*time.Millisecond, 10*time.Millisecond, []int{5})

	var wg sync.WaitGroup
	limiter := NewMulti(policy)
	waitPermission := func() bool {
		permitted, err := limiter.WaitPermission([]int{1})
		if err != nil {
			panic(err)
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

func TestMultiRateLimiter(t *testing.T) {
	policy := NewMultiPolicy(50*time.Millisecond, 10*time.Millisecond, []int{5})

	limiter := NewMulti(policy)
	limiter.SetState(StateNormal)
	for i := 0; i < 30; i++ {
		permitted, d, err := limiter.AcquirePermission([]int{1})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod[0])*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	limiter.SetState(StateDisabled)
	permitted, d, err := limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if !permitted {
		t.Errorf("AcquirePermission should succeeded")
	} else if d != 0 {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	limiter.SetState(StateNormal)
	for i := 0; i < 30; i++ {
		permitted, d, err := limiter.AcquirePermission([]int{1})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
		if d != time.Duration(i/policy.LimitForPeriod[0])*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	limiter.SetState(StateLimiting)
	permitted, d, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	limiter.SetState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	permitted, d, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		permitted, d, err := limiter.AcquirePermission([]int{1})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != policy.TimeoutDuration-time.Millisecond {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	permitted, d, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 30; i++ {
		permitted, d, err := limiter.AcquirePermission([]int{1})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		} else if d != time.Duration(i/policy.LimitForPeriod[0])*policy.LimitRefreshPeriod {
			t.Errorf("wait duration of %d should not be: %s", i, d.String())
		}
	}

	permitted, d, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	} else if d != policy.TimeoutDuration {
		t.Errorf("wait duration should not be: %s", d.String())
	}
	limiter.SetState(StateDisabled)
}

func TestMultiRateLimiter2(t *testing.T) {
	policy := NewMultiPolicy(0, 10*time.Millisecond, []int{10, 50})

	limiter := NewMulti(policy)

	// both exceed rate
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{1, 5})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{1, 5})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if permitted {
			t.Errorf("AcquirePermission should fail: %d", i)
		}
	}

	// second exceed rate
	now = now.Add(time.Millisecond * 10)
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{0, 5})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{1, 0})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if permitted {
			t.Errorf("AcquirePermission should fail: %d", i)
		}
	}

	// first exceed rate
	now = now.Add(time.Millisecond * 10)
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{1, 0})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{0, 5})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if permitted {
			t.Errorf("AcquirePermission should fail: %d", i)
		}
	}

	// both not exceed rate
	now = now.Add(time.Millisecond * 10)
	for i := 0; i < 100; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{0, 0})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}
}

func TestMultiRateLimiterN(t *testing.T) {
	policy := NewMultiPolicy(50*time.Millisecond, 10*time.Millisecond, []int{5})

	limiter := NewMulti(policy)
	limiter.SetState(StateNormal)
	for i := 0; i < 15; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{2})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateDisabled)
	permitted, _, err := limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if !permitted {
		t.Errorf("AcquirePermission should succeeded")
	}

	limiter.SetState(StateNormal)
	for i := 0; i < 10; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{3})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	limiter.SetState(StateLimiting)
	permitted, _, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	}

	limiter.SetState(StateLimiting)
	now = now.Add(time.Millisecond * 5)
	permitted, _, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 6)
	for i := 0; i < 5; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{1})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	permitted, _, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	}

	now = now.Add(time.Millisecond * 89)
	for i := 0; i < 5; i++ {
		permitted, _, err := limiter.AcquirePermission([]int{6})
		if err != nil {
			t.Errorf("AcquirePermission fail, %v", err)
		}
		if !permitted {
			t.Errorf("AcquirePermission should succeed: %d", i)
		}
	}

	permitted, _, err = limiter.AcquirePermission([]int{1})
	if err != nil {
		t.Errorf("AcquirePermission fail, %v", err)
	}
	if permitted {
		t.Errorf("AcquirePermission should fail")
	}
	limiter.SetState(StateDisabled)
}
