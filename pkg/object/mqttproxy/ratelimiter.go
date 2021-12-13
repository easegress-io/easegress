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
	rateLimiter  *ratelimiter.RateLimiter
	burstLimiter *ratelimiter.RateLimiter
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
		l := ratelimiter.New(policy)
		limiter.rateLimiter = l
	}
	if spec.BytesRate > 0 {
		policy := ratelimiter.NewPolicy(0, time.Duration(timePeriod)*time.Second, spec.BytesRate)
		l := ratelimiter.New(policy)
		limiter.burstLimiter = l
	}
	return limiter
}

func (l *Limiter) acquirePermission(burst int) bool {
	permitted := true
	if l.rateLimiter != nil {
		permitted, _ = l.rateLimiter.AcquirePermission()
	}
	if !permitted {
		return permitted
	}
	if l.burstLimiter != nil {
		permitted, _ = l.burstLimiter.AcquireNPermission(burst)
	}
	return permitted
}
