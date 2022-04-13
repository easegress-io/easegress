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

var circuitBreakKind = &Kind{
	Name: "CircuitBreaker",
	DefaultPolicy: func() Policy {
		return &CircuitBreakPolicy{}
	},
}

var _ Policy = (*CircuitBreakPolicy)(nil)

type CircuitBreakPolicy struct {
	BaseSpec
	SlidingWindowType                string `yaml:"slidingWindowType"  jsonschema:"omitempty,enum=COUNT_BASED,enum=TIME_BASED"`
	FailureRateThreshold             uint8  `yaml:"failureRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
	SlowCallRateThreshold            uint8  `yaml:"slowCallRateThreshold" jsonschema:"omitempty,minimum=1,maximum=100"`
	CountingNetworkError             bool   `yaml:"countingNetworkError" jsonschema:"omitempty"`
	SlidingWindowSize                uint32 `yaml:"slidingWindowSize" jsonschema:"omitempty,minimum=1"`
	PermittedNumberOfCallsInHalfOpen uint32 `yaml:"permittedNumberOfCallsInHalfOpenState" jsonschema:"omitempty"`
	MinimumNumberOfCalls             uint32 `yaml:"minimumNumberOfCalls" jsonschema:"omitempty"`
	SlowCallDurationThreshold        string `yaml:"slowCallDurationThreshold" jsonschema:"omitempty,format=duration"`
	MaxWaitDurationInHalfOpen        string `yaml:"maxWaitDurationInHalfOpenState" jsonschema:"omitempty,format=duration"`
	WaitDurationInOpen               string `yaml:"waitDurationInOpenState" jsonschema:"omitempty,format=duration"`
}

func (p *CircuitBreakPolicy) Validate() error {
	// TODO
	return nil
}
