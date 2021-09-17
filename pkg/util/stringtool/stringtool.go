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

package stringtool

import (
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
