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

package ipfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIPFilterChain(t *testing.T) {
	assert := assert.New(t)

	assert.Nil(NewIPFilterChain(nil, nil))

	filters := NewIPFilterChain(nil, &Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	})
	assert.NotNil(filters)

	assert.NotNil(NewIPFilterChain(filters, nil))
}

func TestNewIPFilter(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(New(nil))
	assert.NotNil(New(&Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	}))
}

func TestAllowIP(t *testing.T) {
	assert := assert.New(t)
	var filter *IPFilter
	assert.True(filter.Allow("192.168.1.1"))
	filter = New(&Spec{
		AllowIPs: []string{"192.168.1.0/24"},
		BlockIPs: []string{"192.168.2.0/24"},
	})
	assert.True(filter.Allow("192.168.1.1"))
	assert.False(filter.Allow("192.168.2.1"))
}
