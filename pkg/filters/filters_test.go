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

	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

type mockSpec struct {
	BaseSpec `json:",inline"`
	Field    string `json:"field" jsonschema:"required"`
}

var mockKind = &Kind{
	Name:           "Mock",
	Description:    "none",
	Results:        []string{},
	DefaultSpec:    func() Spec { return &mockSpec{} },
	CreateInstance: func(spec Spec) Filter { return nil },
}

func TestSpecInherit(t *testing.T) {
	type DerivedSpec struct {
		BaseSpec `json:",inline"`
		Field1   string `json:"field1"`
	}

	text := []byte(`
kind: Kind1
name: Name1
field1: abc
`)

	var spec Spec
	derived := &DerivedSpec{}
	spec = derived
	codectool.MustUnmarshal(text, spec)

	baseSpec := spec.baseSpec()
	baseSpec.pipeline = "pipeline1"

	if derived.Kind() != "Kind1" {
		t.Error("wrong kind")
	}

	if derived.Name() != "Name1" {
		t.Error("wrong name")
	}

	if derived.Field1 != "abc" {
		t.Error("wrong field")
	}

	if derived.Pipeline() != "pipeline1" {
		t.Error("wrong pipeline")
	}
}

func TestNewSpec(t *testing.T) {
	assert := assert.New(t)

	_, err := NewSpec(nil, "pipelin1", func() {})
	assert.NotNil(err, "marshal func should return err")

	_, err = NewSpec(nil, "pipeline1", "invalid spec")
	assert.NotNil(err, "unmarshal 'invalid spec' to MetaSpec should return err")

	yamlConfig := `
name: filter
kind: Filter
`
	rawSpec := map[string]interface{}{}
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	_, err = NewSpec(nil, "pipeline1", rawSpec)
	assert.NotNil(err, "kind Filter not exist")

	// spec that work
	kinds["Mock"] = mockKind
	defer delete(kinds, "Mock")
	yamlConfig = `
name: filter
kind: Mock
field: "abc"
`
	rawSpec = map[string]interface{}{}
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, err := NewSpec(nil, "pipeline1", rawSpec)
	assert.Nil(err)
	assert.Nil(spec.Super())
	assert.NotEmpty(spec.JSONConfig())
}
