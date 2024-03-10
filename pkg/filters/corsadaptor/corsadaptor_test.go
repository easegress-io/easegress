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

package corsadaptor

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
}

func TestCORSAdaptor(t *testing.T) {
	assert := assert.New(t)

	t.Run("CORS preflight-request", func(t *testing.T) {
		const yamlConfig = `
kind: CORSAdaptor
name: cors
`
		rawSpec := make(map[string]interface{})
		codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)

		spec, e := filters.NewSpec(nil, "", rawSpec)
		if e != nil {
			t.Errorf("unexpected error: %v", e)
		}

		cors := kind.CreateInstance(spec)
		cors.Init()

		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodOptions, "http://example.com/", nil)
		req.Header.Set("Origin", "http://example.com")
		assert.Nil(err)
		setRequest(t, ctx, req)

		result := cors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}

		req.Header.Add("Access-Control-Request-Method", "abc")
		result = cors.Handle(ctx)
		if result != resultPreflighted {
			t.Error("request should be preflighted")
		}

		spec, _ = filters.NewSpec(nil, "", rawSpec)
		newCors := kind.CreateInstance(spec)
		newCors.Inherit(cors)
		cors.Close()
		req.Method = http.MethodGet
		result = newCors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}
	})

	t.Run("CORS request", func(t *testing.T) {
		const yamlConfig = `
kind: CORSAdaptor
name: cors
allowedOrigins:
  - test.orig.test
`
		rawSpec := make(map[string]interface{})
		codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)

		spec, e := filters.NewSpec(nil, "", rawSpec)
		if e != nil {
			t.Errorf("unexpected error: %v", e)
		}

		cors := kind.CreateInstance(spec)
		cors.Init()
		cors.Status()

		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodOptions, "http://example.com", nil)
		assert.Nil(err)
		setRequest(t, ctx, req)

		result := cors.Handle(ctx)
		if result != "" {
			t.Error("request should not be processed")
		}

		req.Header.Set("Origin", "test.orig.test")
		req.Header.Add("Access-Control-Request-Method", "get")
		result = cors.Handle(ctx)
		if result != resultPreflighted {
			t.Error("request should be preflighted")
		}

		req.Method = http.MethodGet
		req.Header.Set("Origin", "test.orig.test")
		result = cors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}

		req.Header.Set("Origin", "test1.orig.test")
		result = cors.Handle(ctx)
		if result != resultRejected {
			t.Error("request should be rejected")
		}
	})
}
