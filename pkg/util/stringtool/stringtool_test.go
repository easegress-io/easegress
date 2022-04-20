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
	"testing"

	"github.com/stretchr/testify/assert"
)

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
