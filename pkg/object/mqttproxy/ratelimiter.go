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

package mqttproxy

import (
	"time"

	"github.com/megaease/easegress/pkg/util/ratelimiter"
)

// Limiter is a rate limiter for MQTTProxy
type Limiter struct {
	requestLimiter *ratelimiter.UnlockedRateLimiter
	byteLimiter    *ratelimiter.UnlockedRateLimiter
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
	if spec.RequestRate > 0 {
		policy := ratelimiter.NewPolicy(0, time.Duration(timePeriod)*time.Second, spec.RequestRate)
		l := ratelimiter.NewUnlocked(policy)
		limiter.requestLimiter = l
	}
	if spec.BytesRate > 0 {
		policy := ratelimiter.NewPolicy(0, time.Duration(timePeriod)*time.Second, spec.BytesRate)
		l := ratelimiter.NewUnlocked(policy)
		limiter.byteLimiter = l
	}
	return limiter
}

func (l *Limiter) acquirePermission(byteNum int) bool {
	requestPermit := true
	bytePermit := true
	if l.requestLimiter != nil {
		l.requestLimiter.Lock()
		defer l.requestLimiter.Unlock()
		requestPermit, _ = l.requestLimiter.CheckPermission(1)
	}
	if l.byteLimiter != nil {
		l.byteLimiter.Lock()
		defer l.byteLimiter.Unlock()
		bytePermit, _ = l.byteLimiter.CheckPermission(byteNum)
	}
	if requestPermit && bytePermit {
		if l.requestLimiter != nil {
			l.requestLimiter.DoLastCheck()
		}
		if l.byteLimiter != nil {
			l.byteLimiter.DoLastCheck()
		}
		return true
	}
	return false
}
