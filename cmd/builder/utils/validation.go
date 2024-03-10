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

package utils

import "unicode"

func ValidVariableName(name string) bool {
	if len(name) == 0 {
		return false
	}

	for i, c := range name {
		if i == 0 {
			if !unicode.IsLetter(c) && c != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				return false
			}
		}
	}
	return true
}

func ExportableVariableName(name string) bool {
	if len(name) == 0 {
		return false
	}
	nameArr := []rune(name)
	if !unicode.IsUpper(nameArr[0]) {
		return false
	}
	return ValidVariableName(string(nameArr))
}
