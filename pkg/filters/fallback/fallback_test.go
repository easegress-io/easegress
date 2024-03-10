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

package fallback

import (
	"io"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestFallback(t *testing.T) {
	assert := assert.New(t)
	const yamlConfig = `
kind: Fallback
name: fallback
mockCode: 203
mockHeaders:
  X-Mocked: yes
mockBody: "mocked body"
`
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	fb := kind.CreateInstance(spec)
	fb.Init()
	assert.Equal("fallback", fb.Name())
	assert.Equal(kind, fb.Kind())
	assert.Equal(spec, fb.Spec())

	ctx := context.New(tracing.NoopSpan)
	resp, err := httpprot.NewResponse(nil)
	assert.Nil(err)
	ctx.SetInputResponse(resp)

	fb.Handle(ctx)
	if resp.StatusCode() != 203 {
		t.Error("status code is not correct")
	}
	payload, err := io.ReadAll(resp.GetPayload())
	assert.Nil(err)
	if string(payload) != "mocked body" {
		t.Error("body is not correct")
	}
	if resp.Header().Get("X-Mocked") != "yes" {
		t.Error("header is not correct")
	}

	if fb.Status() != nil {
		t.Error("behavior changed, please update this case")
	}

	spec, _ = filters.NewSpec(nil, "", rawSpec)
	newFb := kind.CreateInstance(spec)
	newFb.Inherit(fb)
	fb.Close()

	resp, err = httpprot.NewResponse(nil)
	assert.Nil(err)
	ctx.SetInputResponse(resp)

	newFb.Handle(ctx)
	if resp.StatusCode() != 203 {
		t.Error("status code is not correct")
	}

	payload, err = io.ReadAll(resp.GetPayload())
	assert.Nil(err)
	if string(payload) != "mocked body" {
		t.Error("body is not correct")
	}
	if resp.Header().Get("X-Mocked") != "yes" {
		t.Error("header is not correct")
	}
}
