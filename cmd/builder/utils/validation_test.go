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

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidVariableName(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name  string
		valid bool
	}{
		{name: "", valid: false},
		{name: "_123", valid: true},
		{name: "Abc", valid: true},
		{name: "acdF", valid: true},
		{name: "!123", valid: false},
		{name: "cx!!", valid: false},
		{name: "myVar", valid: true},
		{name: "my_var", valid: true},
		{name: "_myVar", valid: true},
		{name: "myVar123", valid: true},
		{name: "123myVar", valid: false},
		{name: "my-Var", valid: false},
		{name: "my Var", valid: false},
		{name: "my$Var", valid: false},
		{name: "MyVar", valid: true},
		{name: "longVariableName", valid: true},
		{name: "v", valid: true},
		{name: "_", valid: true},
		{name: "myVar!", valid: false},
		{name: "my..Var", valid: false},
		{name: "", valid: false},
		{name: "my_Var", valid: true},
		{name: "My_Var_123", valid: true},
		{name: "my__Var", valid: true},
		{name: "MYVAR", valid: true},
		{name: "myvar", valid: true},
		{name: "MyVar123_", valid: true},
		{name: "my_Var_123_", valid: true},
		{name: "1_", valid: false},
	}
	for _, tc := range testCases {
		assert.Equal(tc.valid, ValidVariableName(tc.name), fmt.Sprintf("case: %s", tc.name))
	}
}

func TestCapitalVariableName(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name  string
		valid bool
	}{
		{name: "", valid: false},
		{name: "_123", valid: false},
		{name: "Abc", valid: true},
		{name: "AcdF", valid: true},
		{name: "!123", valid: false},
		{name: "cx!!", valid: false},
		{name: "myVar", valid: false},
		{name: "My_var", valid: true},
		{name: "_myVar", valid: false},
		{name: "VyVar123", valid: true},
		{name: "123myVar", valid: false},
		{name: "My-Var", valid: false},
		{name: "My Var", valid: false},
		{name: "My$Var", valid: false},
		{name: "MyVar", valid: true},
		{name: "longVariableName", valid: false},
		{name: "v", valid: false},
		{name: "_", valid: false},
		{name: "MyVar!", valid: false},
		{name: "My..Var", valid: false},
		{name: "", valid: false},
		{name: "my_Var", valid: false},
		{name: "My_Var_123", valid: true},
		{name: "my__Var", valid: false},
		{name: "MYVAR", valid: true},
		{name: "Myvar", valid: true},
		{name: "MyVar123_", valid: true},
		{name: "My_Var_123_", valid: true},
		{name: "1_", valid: false},
		{name: "F1", valid: true},
	}
	for _, tc := range testCases {
		assert.Equal(tc.valid, ExportableVariableName(tc.name), fmt.Sprintf("case: %s", tc.name))
	}
}
