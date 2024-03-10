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

package resilience

// kinds is the resilience kind registry.
var kinds = map[string]*Kind{
	CircuitBreakerKind.Name: CircuitBreakerKind,
	RetryKind.Name:          RetryKind,
}

// WalkKind walks the registry, calling fn for each filter kind, and stops
// walking if fn returns false. fn should never modify its parameter.
func WalkKind(fn func(k *Kind) bool) {
	for _, k := range kinds {
		if !fn(k) {
			break
		}
	}
}

// GetKind gets the filter kind by name, the caller should never modify the
// return value.
func GetKind(name string) *Kind {
	return kinds[name]
}
