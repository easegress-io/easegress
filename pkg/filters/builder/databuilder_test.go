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

package builder

import (
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func getDataBuilder(spec *DataBuilderSpec) *DataBuilder {
	inst := dataBuilderKind.CreateInstance(spec)
	inst.Init()
	return inst.(*DataBuilder)
}

func TestDataBuilder(t *testing.T) {
	assert := assert.New(t)

	// test empty data
	yamlConfig := `
dataKey: testKey
template: |
  {}
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)

		db := dataBuilderKind.CreateInstance(spec).(*DataBuilder)
		db.Inherit(getDataBuilder(spec))

		assert.NoError(db.Spec().(*DataBuilderSpec).Validate())
		defer db.Close()

		assert.Equal(dataBuilderKind, db.Kind())
		ctx := context.New(nil)

		res := db.Handle(ctx)
		assert.Empty(res)

		assert.Equal(map[string]interface{}{}, ctx.GetData("testKey"))
	}

	// test sample data
	yamlConfig = `
dataKey: testKey
template: |
  {
    "name": "Bob",
    "age": 18,
    "address": {
      "city": "Beijing",
      "street": "Changan street"
    }
  }
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		db := getDataBuilder(spec)
		db.Init()
		assert.NoError(db.spec.Validate())
		defer db.Close()

		assert.Equal(dataBuilderKind, db.Kind())
		assert.Equal(db.Name(), spec.Name())
		ctx := context.New(nil)

		res := db.Handle(ctx)
		assert.Empty(res)

		data := ctx.GetData("testKey")
		assert.IsType(map[string]interface{}{}, data)
		assert.EqualValues(map[string]interface{}{"name": "Bob", "age": 18, "address": map[string]interface{}{"city": "Beijing", "street": "Changan street"}}, data)
	}

	// test array
	yamlConfig = `
dataKey: testKey
template: |
  [{
    "name": "Bob",
    "age": 18,
    "address": {
      "city": "Beijing",
      "street": "Changan street"
    }
  },
  {
    "name": "Alice",
    "age": 20,
    "address": {
      "city": "Shanghai",
      "street": "Xuhui street"
    }
  }]
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		db := getDataBuilder(spec)

		db.Init()
		assert.NoError(db.spec.Validate())
		defer db.Close()

		assert.Equal(dataBuilderKind, db.Kind())
		ctx := context.New(nil)

		res := db.Handle(ctx)
		assert.Empty(res)

		data := ctx.GetData("testKey")
		assert.IsType([]interface{}{}, data)

		var expected = make([]interface{}, 2)
		expected[0] = map[string]interface{}{"name": "Bob", "age": 18, "address": map[string]interface{}{"city": "Beijing", "street": "Changan street"}}
		expected[1] = map[string]interface{}{"name": "Alice", "age": 20, "address": map[string]interface{}{"city": "Shanghai", "street": "Xuhui street"}}
		assert.EqualValues(expected, data)
	}

	// test array
	yamlConfig = `
dataKey: testKey
template: |
  [{
    "name": "Bob",
    "age": 18,
    "address": {
      "city": "Beijing",
      "street": "Changan street"
    },
    "friends": [
      "Alice", "Tom", "Jack"
    ]
  }]
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		db := getDataBuilder(spec)

		db.Init()
		assert.NoError(db.spec.Validate())
		defer db.Close()

		assert.Equal(dataBuilderKind, db.Kind())
		ctx := context.New(nil)

		res := db.Handle(ctx)
		assert.Empty(res)

		data := ctx.GetData("testKey")
		assert.IsType([]interface{}{}, data)

		var expected = make([]interface{}, 1)
		expected[0] = map[string]interface{}{"name": "Bob", "age": 18, "address": map[string]interface{}{"city": "Beijing", "street": "Changan street"}, "friends": []interface{}{"Alice", "Tom", "Jack"}}
		assert.EqualValues(expected, data)
	}
}

func TestDataBuilderSpecValidate(t *testing.T) {
	assert := assert.New(t)

	// dataKey is empty
	yamlConfig := `
template: |
  {}
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		assert.Error(spec.Validate())
	}

	// template is empty
	yamlConfig = `
dataKey: testKey
`
	{
		spec := dataBuilderKind.DefaultSpec().(*DataBuilderSpec)
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		assert.Error(spec.Validate())
	}
}
