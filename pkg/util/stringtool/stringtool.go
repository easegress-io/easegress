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

// Package stringtool provides string utilities.
package stringtool

import (
	"fmt"
	"regexp"
	"strings"
)

// Cat concatenates strings.
// It is intended to used in the core executing path for performance optimization.
// fmt.Printf is still recommended for readability.
func Cat(strs ...string) string {
	n := 0
	for _, s := range strs {
		n += len(s)
	}

	var builder strings.Builder
	builder.Grow(n)
	for _, s := range strs {
		builder.WriteString(s)
	}

	return builder.String()
}

// StrInSlice returns whether the string is in the slice.
func StrInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}

	return false
}

// DeleteStrInSlice deletes the matched string in the slice.
func DeleteStrInSlice(slice []string, str string) []string {
	result := []string{}
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}

	return result
}

// IsAllEmpty returns whether all strings are empty.
func IsAllEmpty(strs ...string) bool {
	if len(strs) == 0 {
		return true
	}

	for _, s := range strs {
		if s != "" {
			return false
		}
	}

	return true
}

// IsAnyEmpty returns whether any string is empty.
func IsAnyEmpty(strs ...string) bool {
	if len(strs) == 0 {
		return false
	}

	for _, s := range strs {
		if s == "" {
			return true
		}
	}

	return false
}

// StringMatcher defines the match rule of a string
type StringMatcher struct {
	Exact  string `json:"exact,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	RegEx  string `json:"regex,omitempty" jsonschema:"format=regexp"`
	Empty  bool   `json:"empty,omitempty"`
	re     *regexp.Regexp
}

// Validate validates the StringMatcher.
func (sm *StringMatcher) Validate() error {
	if sm.Empty {
		if sm.Exact != "" || sm.Prefix != "" || sm.RegEx != "" {
			return fmt.Errorf("empty is conflict with other patterns")
		}
		return nil
	}

	if sm.Exact != "" {
		return nil
	}

	if sm.Prefix != "" {
		return nil
	}

	if sm.RegEx != "" {
		return nil
	}

	return fmt.Errorf("all patterns are empty")
}

// Init initializes the StringMatcher.
func (sm *StringMatcher) Init() {
	if sm.RegEx != "" {
		sm.re = regexp.MustCompile(sm.RegEx)
	}
}

// Match matches a string.
func (sm *StringMatcher) Match(value string) bool {
	if sm.Empty && value == "" {
		return true
	}

	if sm.Exact != "" && value == sm.Exact {
		return true
	}

	if sm.Prefix != "" && strings.HasPrefix(value, sm.Prefix) {
		return true
	}

	if sm.re == nil {
		return false
	}

	return sm.re.MatchString(value)
}

// MatchAny return true if any of the values matches.
func (sm *StringMatcher) MatchAny(values []string) bool {
	for _, v := range values {
		if sm.Match(v) {
			return true
		}
	}
	return false
}
