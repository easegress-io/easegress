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
	"time"

	"github.com/megaease/easegress/v2/pkg/util/ratelimiter"
)

// Limiter is a rate limiter for MQTTProxy
type Limiter struct {
	multiLimiter   *ratelimiter.MultiRateLimiter
	requestLimiter *ratelimiter.RateLimiter
	byteLimiter    *ratelimiter.RateLimiter
}

func newLimiter(spec *RateLimit) *Limiter {
	limiter := &Limiter{}
	if spec == nil || (spec.RequestRate == 0 && spec.BytesRate == 0) {
		return limiter
	}
	timePeriod := 1
	if spec.TimePeriod > 0 {
		timePeriod = spec.TimePeriod
	}
	if spec.RequestRate > 0 && spec.BytesRate > 0 {
		policy := ratelimiter.NewMultiPolicy(0, time.Duration(timePeriod)*time.Second, []int{spec.RequestRate, spec.BytesRate})
		l := ratelimiter.NewMulti(policy)
		limiter.multiLimiter = l

	} else if spec.RequestRate > 0 {
		policy := ratelimiter.NewPolicy(0, time.Duration(timePeriod)*time.Second, spec.RequestRate)
		l := ratelimiter.New(policy)
		limiter.requestLimiter = l

	} else if spec.BytesRate > 0 {
		policy := ratelimiter.NewPolicy(0, time.Duration(timePeriod)*time.Second, spec.BytesRate)
		l := ratelimiter.New(policy)
		limiter.byteLimiter = l
	}
	return limiter
}

func (l *Limiter) acquirePermission(byteNum int) bool {
	if l.multiLimiter != nil {
		permitted, _, _ := l.multiLimiter.AcquirePermission([]int{1, byteNum})
		return permitted
	}
	if l.requestLimiter != nil {
		permitted, _ := l.requestLimiter.AcquirePermission()
		return permitted
	}
	if l.byteLimiter != nil {
		permitted, _ := l.byteLimiter.AcquireNPermission(byteNum)
		return permitted
	}
	return true
}
