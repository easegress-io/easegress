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

func TestResultBuider(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `template: result0`
	{
		spec := resultBuilderKind.DefaultSpec()
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := resultBuilderKind.CreateInstance(spec).(*ResultBuilder)
		rb.Init()
		assert.NotNil(rb.Spec())
		assert.NoError(rb.spec.Validate())
		defer rb.Close()

		assert.Equal(resultBuilderKind, rb.Kind())
		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Equal("result0", res)
	}

	yamlConfig = `template: ""`
	{
		spec := resultBuilderKind.DefaultSpec()
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := resultBuilderKind.CreateInstance(spec)
		rb.Init()
		rb2 := resultBuilderKind.CreateInstance(spec)
		rb2.Inherit(rb)
		rb.Close()
		rb = rb2

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Equal("", res)
	}

	yamlConfig = `template: resultx`
	{
		spec := resultBuilderKind.DefaultSpec()
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := resultBuilderKind.CreateInstance(spec)
		rb.Init()
		defer rb.Close()

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Equal(resultUnknown, res)
	}

	yamlConfig = `template: '{{panic "panicked"}}'`
	{
		spec := resultBuilderKind.DefaultSpec()
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := resultBuilderKind.CreateInstance(spec)
		rb.Init()
		defer rb.Close()

		ctx := context.New(nil)
		res := rb.Handle(ctx)
		assert.Equal(resultBuildErr, res)
	}

	yamlConfig = `template: "{{index .resultx 1}}"`
	{
		spec := resultBuilderKind.DefaultSpec()
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := resultBuilderKind.CreateInstance(spec)
		rb.Init()
		defer rb.Close()

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Equal(resultBuildErr, res)
	}
}
