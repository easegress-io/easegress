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
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func defaultFilterSpec(spec *Spec) *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Name:     "request-adaptor",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
	}
	filterSpec := httppipeline.MockFilterSpec(nil, nil, "", meta, spec)
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

func getTemplate(t *testing.T, filterSpec *httppipeline.FilterSpec) *context.HTTPTemplate {
	filterBuffs := []context.FilterBuff{
		{Name: filterSpec.Name(), Buff: []byte(filterSpec.YAMLConfig())},
	}
	httpTemp, err := context.NewHTTPTemplate(filterBuffs)
	assert.Nil(t, err)
	return httpTemp
}

func TestRequestAdaptor(t *testing.T) {
	assert := assert.New(t)
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

func TestDecompose(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		Decompress: "gzip",
	}
	filterSpec := defaultFilterSpec(spec)
	httpTemp := getTemplate(t, filterSpec)

	ra := &RequestAdaptor{}
	ra.Init(filterSpec)

	{
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

func TestMethod(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		Method: http.MethodDelete,
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
}
