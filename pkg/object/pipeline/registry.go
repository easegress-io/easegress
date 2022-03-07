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

package pipeline

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegress/pkg/context"
)

type (
	// Filter is the common interface for filters handling HTTP traffic.
	Filter interface {
		// Kind returns the unique kind name to represent itself.
		Kind() string

		// DefaultSpec returns the default spec.
		DefaultSpec() interface{}

		// Description returns the description of the filter.
		Description() string

		// Results returns all possible results, the normal result
		// (i.e. empty string) could not be in it.
		Results() []string

		// Init initializes the Filter.
		Init(filterSpec *FilterSpec)

		// Inherit also initializes the Filter.
		// But it needs to handle the lifecycle of the previous generation.
		// So it's own responsibility for the filter to inherit and clean the previous generation stuff.
		// The http pipeline won't call Close for the previous generation.
		Inherit(filterSpec *FilterSpec, previousGeneration Filter)

		// Handle handles one HTTP request, all possible results
		// need be registered in Results.
		Handle(context.HTTPContext) (result string)

		// Status returns its runtime status.
		// It could return nil.
		Status() interface{}

		// Close closes itself.
		Close()
	}
)

var filterRegistry = map[string]Filter{}

// Register registers filter.
func Register(f Filter) {
	if f.Kind() == "" {
		panic(fmt.Errorf("%T: empty kind", f))
	}

	existedFilter, existed := filterRegistry[f.Kind()]
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

	filterRegistry[f.Kind()] = f
}

// GetFilterRegistry get the filter registry.
func GetFilterRegistry() map[string]Filter {
	result := map[string]Filter{}

	for kind, f := range filterRegistry {
		result[kind] = f
	}

	return result
}
