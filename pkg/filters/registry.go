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

package filters

import (
	"fmt"
	"sort"
)

// kinds is the filter kind registry.
var kinds = map[string]*Kind{}

// Register registers a filter kind.
func Register(k *Kind) {
	if k.Name == "" {
		panic(fmt.Errorf("%T: empty kind name", k))
	}

	if k1 := kinds[k.Name]; k1 != nil {
		msgFmt := "%T and %T got same name: %s"
		panic(fmt.Errorf(msgFmt, k, k1, k.Name))
	}

	sort.Strings(k.Results)

	// Checking results.
	resultMap := map[string]struct{}{}
	for _, result := range k.Results {
		if _, ok := resultMap[result]; ok {
			panic(fmt.Errorf("duplicated result: %s", result))
		}
		resultMap[result] = struct{}{}
	}

	kinds[k.Name] = k
}

// Unregister unregisters a filter kind, mainly for testing purpose.
func Unregister(name string) {
	delete(kinds, name)
}

// ResetRegistry reset the filter kind registry, mainly for testing purpose.
func ResetRegistry() {
	kinds = map[string]*Kind{}
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

// Create creates a filter instance of kind.
func Create(spec Spec) Filter {
	k := kinds[spec.Kind()]
	if k == nil {
		return nil
	}
	return k.CreateInstance(spec)
}
