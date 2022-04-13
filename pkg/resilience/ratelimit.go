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

package resilience

var rateLimitKind = &Kind{
	Name: "RateLimiter",
	DefaultPolicy: func() Policy {
		return &RateLimitPolicy{}
	},
}

var _ Policy = (*RateLimitPolicy)(nil)

type RateLimitPolicy struct {
	BaseSpec
	TimeoutDuration    string `yaml:"timeoutDuration" jsonschema:"omitempty,format=duration"`
	LimitRefreshPeriod string `yaml:"limitRefreshPeriod" jsonschema:"omitempty,format=duration"`
	LimitForPeriod     int    `yaml:"limitForPeriod" jsonschema:"omitempty,minimum=1"`
}

func (p *RateLimitPolicy) Validate() error {
	// TODO
	return nil
}
