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
	"testing"

	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

var duplicatedResult = &Kind{
	Name:           "DuplicatedResult",
	Description:    "none",
	Results:        []string{"1", "2", "1"},
	DefaultSpec:    func() Spec { return &mockSpec{} },
	CreateInstance: func(spec Spec) Filter { return nil },
}

func TestRegister(t *testing.T) {
	assert := assert.New(t)

	// test Unregister
	Register(mockKind)
	assert.NotNil(GetKind(mockKind.Name))
	Unregister(mockKind.Name)
	assert.Nil(GetKind(mockKind.Name))

	// test ResetRegistry
	Register(mockKind)
	assert.NotNil(GetKind(mockKind.Name))
	ResetRegistry()
	assert.Nil(GetKind(mockKind.Name))

	// test Register panic with invalid kind
	Register(mockKind)
	assert.Panics(func() { Register(mockKind) })
	assert.Panics(func() { Register(duplicatedResult) })

	baseSpec := &BaseSpec{
		MetaSpec: supervisor.MetaSpec{
			Kind: mockKind.Name,
		},
	}
	assert.Nil(Create(baseSpec))

	WalkKind(func(k *Kind) bool {
		assert.Equal(mockKind.Name, k.Name)
		return false
	})

	k := &Kind{}
	assert.Panics(func() { Register(k) })

	assert.Nil(Create(&BaseSpec{
		MetaSpec: supervisor.MetaSpec{
			Kind: "UnRegistryKind",
		},
	}))
}
