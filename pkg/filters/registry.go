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

package filters

import (
	"fmt"
	"reflect"
)

var registry = map[string]Filter{}

// Register registers filter.
func Register(f Filter) {
	if f.Kind() == "" {
		panic(fmt.Errorf("%T: empty kind", f))
	}

	existedFilter, existed := registry[f.Kind()]
	if existed {
		panic(fmt.Errorf("%T and %T got same kind: %s", f, existedFilter, f.Kind()))
	}

	// Checking filter type.
	filterType := reflect.TypeOf(f)
	if filterType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s: want a pointer, got %s", f.Kind(), filterType.Kind()))
	}
	if filterType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s elem: want a struct, got %s", f.Kind(), filterType.Kind()))
	}

	// Checking spec type.
	specType := reflect.TypeOf(f.DefaultSpec())
	if specType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s spec: want a pointer, got %s", f.Kind(), specType.Kind()))
	}
	if specType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s spec elem: want a struct, got %s", f.Kind(), specType.Elem().Kind()))
	}

	// Checking results.
	results := make(map[string]struct{})
	for _, result := range f.Results() {
		_, exists := results[result]
		if exists {
			panic(fmt.Errorf("repeated result: %s", result))
		}
		results[result] = struct{}{}
	}

	registry[f.Kind()] = f
}

// Unregister unregisters a filter kind, mainly for testing purpose.
func Unregister(kind string) {
	delete(registry, kind)
}

// ResetRegistry reset the filter registry, mainly for testing purpose.
func ResetRegistry() {
	registry = map[string]Filter{}
}

// Registry returns the filter registry.
func Registry() map[string]Filter {
	result := map[string]Filter{}

	for kind, f := range registry {
		result[kind] = f
	}

	return result
}

// GetRoot returns the root filter from registry by kind.
func GetRoot(kind string) Filter {
	return registry[kind]
}

// Create creates an filter instance of kind 'kind'
func Create(kind string) Filter {
	root := registry[kind]
	if root == nil {
		return nil
	}
	return reflect.New(reflect.TypeOf(root).Elem()).Interface().(Filter)
}
