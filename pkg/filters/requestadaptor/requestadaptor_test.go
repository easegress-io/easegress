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

package requestadaptor

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func defaultFilterSpec(spec *Spec) *pipeline.FilterSpec {
	meta := &supervisor.MetaSpec{
		Name: "request-adaptor",
		Kind: Kind,
	}
	filterSpec := pipeline.MockFilterSpec(nil, "", meta, spec, "pipeline-demo")
	return filterSpec
}

func getGzipEncoding(t *testing.T, data []byte) io.Reader {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	_, err := gw.Write(data)
	assert.Nil(t, err)
	return &buf
}

func getContext(t *testing.T, req *http.Request, httpTemp *context.HTTPTemplate) context.HTTPContext {
	w := httptest.NewRecorder()
	ctx := context.New(w, req, tracing.NoopTracing, "no trace")
	ctx.SetHandlerCaller(func(lastResult string) string {
		return lastResult
	})
	ctx.SetTemplate(httpTemp)
	return ctx
}

func getTemplate(t *testing.T, filterSpec *pipeline.FilterSpec) *context.HTTPTemplate {
	filterBuffs := []context.FilterBuff{
		{Name: filterSpec.Name(), Buff: []byte(filterSpec.YAMLConfig())},
	}
	httpTemp, err := context.NewHTTPTemplate(filterBuffs)
	assert.Nil(t, err)
	return httpTemp
}

func TestRequestAdaptor(t *testing.T) {
	assert := assert.New(t)
	{
		// normal case
		spec := &Spec{}
		filterSpec := defaultFilterSpec(spec)
		ra := &RequestAdaptor{}
		ra.Init(filterSpec)
		assert.Equal(Kind, ra.Kind())
		assert.NotEmpty(ra.Description())
		assert.NotNil(ra.DefaultSpec())
		assert.NotEmpty(ra.Results())
		assert.Nil(ra.Status())

		newRA := &RequestAdaptor{}
		newRA.Inherit(filterSpec, ra)
		newRA.Close()
	}

	{
		// panic when invalid compress type
		spec := &Spec{
			Compress: "zip",
		}
		filterSpec := defaultFilterSpec(spec)
		ra := &RequestAdaptor{}
		assert.Panics(func() { ra.Init(filterSpec) })
	}

	{
		// panic when invalid decompress type
		spec := &Spec{
			Decompress: "zip",
		}
		filterSpec := defaultFilterSpec(spec)
		ra := &RequestAdaptor{}
		assert.Panics(func() { ra.Init(filterSpec) })
	}

	{
		// panic when set both set compress and decompress
		spec := &Spec{
			Decompress: "gzip",
			Compress:   "gzip",
		}
		filterSpec := defaultFilterSpec(spec)
		ra := &RequestAdaptor{}
		assert.Panics(func() { ra.Init(filterSpec) })
	}

	{
		// panic when set body and Decompress
		spec := &Spec{
			Decompress: "gzip",
			Body:       "body",
		}
		filterSpec := defaultFilterSpec(spec)
		ra := &RequestAdaptor{}
		assert.Panics(func() { ra.Init(filterSpec) })
	}
}

func TestDecompress(t *testing.T) {
	assert := assert.New(t)

	{
		// decompress without body in spec
		spec := &Spec{
			Decompress: "gzip",
		}
		filterSpec := defaultFilterSpec(spec)
		httpTemp := getTemplate(t, filterSpec)

		ra := &RequestAdaptor{}
		ra.Init(filterSpec)

		{
			// compress success
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", getGzipEncoding(t, []byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := getContext(t, req, httpTemp)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)
			ctx.Finish()

			encoding := ctx.Request().Header().Get("Content-Encoding")
			assert.Equal("", encoding)

			body, err := io.ReadAll(ctx.Request().Body())
			assert.Nil(err)
			assert.Equal("123", string(body))
		}

		{
			// compress fail
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader([]byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := getContext(t, req, httpTemp)

			ans := ra.Handle(ctx)
			assert.Equal(resultDecompressFail, ans)
			ctx.Finish()
		}
	}
}

func TestCompress(t *testing.T) {
	assert := assert.New(t)

	{
		// compress without body in spec
		spec := &Spec{
			Compress: "gzip",
		}
		filterSpec := defaultFilterSpec(spec)
		httpTemp := getTemplate(t, filterSpec)

		ra := &RequestAdaptor{}
		ra.Init(filterSpec)

		data := "123"
		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader([]byte(data)))
		assert.Nil(err)

		ctx := getContext(t, req, httpTemp)

		ans := ra.Handle(ctx)
		assert.Equal("", ans)
		ctx.Finish()

		encoding := ctx.Request().Header().Get("Content-Encoding")
		assert.Equal("gzip", encoding)

		reader, err := gzip.NewReader(ctx.Request().Body())
		assert.Nil(err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		assert.Nil(err)
		assert.Equal("123", string(body))
	}

	{
		// compress with body in spec
		spec := &Spec{
			Body:     "spec_body",
			Compress: "gzip",
		}
		filterSpec := defaultFilterSpec(spec)
		httpTemp := getTemplate(t, filterSpec)

		ra := &RequestAdaptor{}
		ra.Init(filterSpec)

		{
			// original gzip body
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", getGzipEncoding(t, []byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := getContext(t, req, httpTemp)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)
			ctx.Finish()

			encoding := ctx.Request().Header().Get("Content-Encoding")
			assert.Equal("gzip", encoding)

			reader, err := gzip.NewReader(ctx.Request().Body())
			assert.Nil(err)
			defer reader.Close()
			body, err := io.ReadAll(reader)
			assert.Nil(err)
			assert.Equal("spec_body", string(body))
		}

		{
			// original uncompressed body
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader([]byte(data)))
			assert.Nil(err)

			ctx := getContext(t, req, httpTemp)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)
			ctx.Finish()

			encoding := ctx.Request().Header().Get("Content-Encoding")
			assert.Equal("gzip", encoding)

			reader, err := gzip.NewReader(ctx.Request().Body())
			assert.Nil(err)
			defer reader.Close()
			body, err := io.ReadAll(reader)
			assert.Nil(err)
			assert.Equal("spec_body", string(body))
		}
	}
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		Method: http.MethodDelete,
		Host:   "127.0.0.2",
		Body:   "123",
		Header: &httpheader.AdaptSpec{
			Add: map[string]string{"X-Add": "add-value"},
			Set: map[string]string{"X-Set": "set-value"},
		},
		Path: &pathadaptor.Spec{
			Replace: "/path",
		},
	}
	filterSpec := defaultFilterSpec(spec)
	httpTemp := getTemplate(t, filterSpec)

	ra := &RequestAdaptor{}
	ra.Init(filterSpec)

	req, err := http.NewRequest(http.MethodPost, "127.0.0.1", nil)
	assert.Nil(err)

	ctx := getContext(t, req, httpTemp)

	ans := ra.Handle(ctx)
	assert.Equal("", ans)
	ctx.Finish()

	method := ctx.Request().Method()
	assert.Equal(http.MethodDelete, method)

	host := ctx.Request().Host()
	assert.Equal("127.0.0.2", host)

	body, err := io.ReadAll(ctx.Request().Body())
	assert.Nil(err)
	assert.Equal("123", string(body))

	headerValue := ctx.Request().Header().Get("X-Add")
	assert.Equal("add-value", headerValue)

	headerValue = ctx.Request().Header().Get("X-Set")
	assert.Equal("set-value", headerValue)

	path := ctx.Request().Path()
	assert.Equal("/path", path)
}
