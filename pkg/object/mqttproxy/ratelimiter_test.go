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

package mqttproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter(t *testing.T) {
	// default spec will not block anything
	spec := &RateLimit{}
	l := newLimiter(spec)
	for i := 0; i < 1000; i++ {
		permitted := l.acquirePermission(100)
		assert.Equal(t, true, permitted)
	}

	// only provide rate should only check rate
	spec = &RateLimit{
		RequestRate: 10,
		TimePeriod:  60,
	}
	l = newLimiter(spec)
	for i := 0; i < 20; i++ {
		permitted := l.acquirePermission(1000)
		if i < 10 {
			assert.Equal(t, true, permitted)
		} else {
			assert.Equal(t, false, permitted)
		}
	}

	// only provide burst should only check burst
	spec = &RateLimit{
		BytesRate:  100,
		TimePeriod: 60,
	}
	l = newLimiter(spec)
	for i := 0; i < 20; i++ {
		permitted := l.acquirePermission(10)
		if i < 10 {
			assert.Equal(t, true, permitted)
		} else {
			assert.Equal(t, false, permitted)
		}
	}

	// not provide time period use one second
	spec = &RateLimit{
		BytesRate: 100,
	}
	l = newLimiter(spec)
	permitted := l.acquirePermission(100)
	assert.Equal(t, true, permitted)
	permitted = l.acquirePermission(1)
	assert.Equal(t, false, permitted)
	time.Sleep(1100 * time.Millisecond)
	permitted = l.acquirePermission(100)
	assert.Equal(t, true, permitted)

	// provide both and use both
	spec = &RateLimit{
		RequestRate: 10,
		BytesRate:   100,
		TimePeriod:  60,
	}
	l = newLimiter(spec)
	for i := 0; i < 20; i++ {
		permitted := l.acquirePermission(10)
		if i < 10 {
			assert.Equal(t, true, permitted)
		} else {
			assert.Equal(t, false, permitted)
		}
	}

	// provide both and use both
	spec = &RateLimit{
		RequestRate: 10,
		BytesRate:   100,
		TimePeriod:  60,
	}
	l = newLimiter(spec)
	for i := 0; i < 20; i++ {
		permitted := l.acquirePermission(0)
		if i < 10 {
			assert.Equal(t, true, permitted)
		} else {
			assert.Equal(t, false, permitted)
		}
	}

	// provide both and use both
	spec = &RateLimit{
		RequestRate: 100,
		BytesRate:   10,
		TimePeriod:  60,
	}
	l = newLimiter(spec)
	for i := 0; i < 20; i++ {
		permitted := l.acquirePermission(1)
		if i < 10 {
			assert.Equal(t, true, permitted)
		} else {
			assert.Equal(t, false, permitted)
		}
	}
}
