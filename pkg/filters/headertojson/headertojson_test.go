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

package headertojson

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func setRequest(t *testing.T, ctx *context.Context, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	err = req.FetchPayload(1024 * 1024)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
}

func defaultFilterSpec(spec *Spec) filters.Spec {
	spec.BaseSpec.MetaSpec.Kind = Kind
	spec.BaseSpec.MetaSpec.Name = "header-to-json"
	result, _ := filters.NewSpec(nil, "pipeline-demo", spec)
	return result
}

func TestHeaderToJSON(t *testing.T) {
	assert := assert.New(t)
	spec := defaultFilterSpec(&Spec{
		HeaderMap: []*HeaderMap{
			{
				Header: "X-Header-1",
				JSON:   "header1",
			},
		},
	})
	h := kind.CreateInstance(spec)
	h.Init()
	assert.Equal(spec.Name(), h.Name())
	assert.Equal(kind, h.Kind())
	assert.Equal(spec, h.Spec())
	assert.Nil(h.Status())

	stdReq, err := http.NewRequest("", "/", nil)
	assert.Nil(err)
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(err)
	ctx := context.New(nil)
	ctx.SetInputRequest(req)
	assert.Equal("", h.Handle(ctx))

	newh := kind.CreateInstance(spec)
	newh.Inherit(h)
	newh.Close()
}

func TestHandleHTTP(t *testing.T) {
	assert := assert.New(t)
	spec := defaultFilterSpec(&Spec{
		HeaderMap: []*HeaderMap{
			{Header: "x-username", JSON: "username"},
		},
	})

	h2j := kind.CreateInstance(spec)
	h2j.Init()

	{
		// test http request with body
		bodyMap := map[string]interface{}{
			"topic": "log",
			"id":    "abc123",
		}
		body, err := json.Marshal(bodyMap)
		assert.Nil(err)

		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(body))
		assert.Nil(err)
		req.Header.Add("x-username", "clientA")

		ctx := context.New(nil)
		setRequest(t, ctx, req)

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)
		ctx.Finish()

		body, err = io.ReadAll(ctx.GetOutputRequest().GetPayload())
		assert.Nil(err)

		res := map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		assert.Equal("log", res["topic"])
		assert.Equal("abc123", res["id"])
		assert.Equal("clientA", res["username"])
	}

	{
		// test http request without body
		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", nil)
		assert.Nil(err)

		req.Header.Add("x-username", "clientA")
		ctx := context.New(nil)
		setRequest(t, ctx, req)

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)

		body, err := io.ReadAll(ctx.GetOutputRequest().GetPayload())
		assert.Nil(err)

		res := map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		assert.Equal("clientA", res["username"])
	}

	{
		// test http request with array body
		bodyMap := []map[string]interface{}{
			{"log": "123", "id": "abc"},
			{"log": "456", "id": "def"},
		}
		body, err := json.Marshal(bodyMap)
		assert.Nil(err)

		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(body))
		assert.Nil(err)
		req.Header.Add("x-username", "clientA")

		ctx := context.New(nil)
		setRequest(t, ctx, req)

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)
		ctx.Finish()

		body, err = io.ReadAll(ctx.GetOutputRequest().GetPayload())
		assert.Nil(err)

		res := []map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		for _, r := range res {
			if r["log"] == "123" {
				assert.Equal("abc", r["id"])
				assert.Equal("clientA", r["username"])

			} else if r["log"] == "456" {
				assert.Equal("def", r["id"])
				assert.Equal("clientA", r["username"])
			} else {
				t.Error("wrong result")
			}
		}
	}

	{
		// test http request with stream body
		stdReq, err := http.NewRequest(http.MethodPost, "127.0.0.1", strings.NewReader("123"))
		assert.Nil(err)
		stdReq.Header.Add("x-username", "clientA")

		req, err := httpprot.NewRequest(stdReq)
		assert.Nil(err)
		req.FetchPayload(-1)
		assert.True(req.IsStream())

		ctx := context.New(nil)
		ctx.SetInputRequest(req)

		ans := h2j.Handle(ctx)
		assert.Equal(resultBodyReadErr, ans)
	}
}
