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

var retryKind = &Kind{
	Name: "Retryer",
	DefaultPolicy: func() Policy {
		return &RetryPolicy{}
	},
}

var _ Policy = (*RetryPolicy)(nil)

type RetryPolicy struct {
	BaseSpec
	MaxAttempts         int     `yaml:"maxAttempts" jsonschema:"omitempty,minimum=1"`
	WaitDuration        string  `yaml:"waitDuration" jsonschema:"omitempty,format=duration"`
	BackOffPolicy       string  `yaml:"backOffPolicy" jsonschema:"omitempty,enum=random,enum=exponential"`
	RandomizationFactor float64 `yaml:"randomizationFactor" jsonschema:"omitempty,minimum=0,maximum=1"`
}

func (p *RetryPolicy) Validate() error {
	// TODO
	return nil
}
