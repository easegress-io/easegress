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

package stringtool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringTool(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("123", Cat("1", "2", "3"))
	assert.True(StrInSlice("123", []string{"000", "111", "123"}))
	assert.False(StrInSlice("123", []string{"000", "111"}))
	assert.Equal([]string{"123"}, DeleteStrInSlice([]string{"123", "456"}, "456"))
}

func TestIsAllEmpty(t *testing.T) {
	assert := assert.New(t)

	assert.True(IsAllEmpty("", "", ""))
	assert.True(IsAllEmpty())
	assert.True(IsAllEmpty([]string{}...))
	assert.False(IsAllEmpty("", "", "a"))
	assert.False(IsAllEmpty("", "a", ""))
	assert.False(IsAllEmpty("a", "", ""))
	assert.False(IsAllEmpty("a", "a", ""))
	assert.False(IsAllEmpty("a", "a", "a"))
	assert.False(IsAllEmpty([]string{"a", "a", "a"}...))
}

func TestIsAnyEmpty(t *testing.T) {
	assert := assert.New(t)

	assert.True(IsAnyEmpty("", "", ""))
	assert.True(IsAnyEmpty("", "", "a"))
	assert.True(IsAnyEmpty("", "a", ""))
	assert.True(IsAnyEmpty("a", "", ""))
	assert.True(IsAnyEmpty("a", "a", ""))

	assert.False(IsAnyEmpty())
	assert.False(IsAnyEmpty([]string{}...))
	assert.False(IsAnyEmpty("a", "a", "a"))
	assert.False(IsAnyEmpty([]string{"a", "a", "a"}...))
}

func TestStringMatcher(t *testing.T) {
	assert := assert.New(t)

	// validation
	sm := &StringMatcher{Empty: true}
	assert.NoError(sm.Validate())
	sm.Init()

	sm = &StringMatcher{Empty: true, Exact: "abc"}
	assert.Error(sm.Validate())

	sm = &StringMatcher{}
	assert.Error(sm.Validate())

	sm = &StringMatcher{RegEx: "^abc[0-9]+$"}
	assert.NoError(sm.Validate())
	sm.Init()

	sm.Prefix = "/xyz"
	assert.NoError(sm.Validate())

	sm.Exact = "/abc"
	assert.NoError(sm.Validate())

	// match
	sm = &StringMatcher{Empty: true}
	assert.True(sm.Match(""))
	assert.False(sm.Match("abc"))

	sm = &StringMatcher{RegEx: "^abc[0-9]+$"}
	sm.Init()
	assert.True(sm.Match("abc123"))
	assert.False(sm.Match("abc123d"))

	sm.Prefix = "/xyz"
	assert.True(sm.Match("/xyz123"))
	assert.False(sm.Match("/Xyz123"))

	sm.Exact = "/hello"
	assert.True(sm.Match("/hello"))
	assert.False(sm.Match("/Hello"))
}
