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

package responseadaptor

import (
	"github.com/megaease/easegress/pkg/filters/builder"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestResponseAdaptor(t *testing.T) {
	assert := assert.New(t)

	yamlSpec := `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "X-Mock": "mockedHeaderValue"
  set:
    "X-Set": "setHeaderValue"
body: "copyright"
`
	ra := doTest(t, yamlSpec, nil)
	assert.Equal("ra", ra.Name())
	assert.Equal(kind, ra.Kind())
	assert.Equal("ResponseAdaptor", ra.Spec().Kind())

	yamlSpec = `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "X-Mock": "mockedHeaderValue"
  set:
    "X-Set": "setHeaderValue"
body: "copyright"
`
	ra = doTest(t, yamlSpec, ra)
	ra.Close()
}

func doTest(t *testing.T, yamlSpec string, prev *ResponseAdaptor) *ResponseAdaptor {
	assert := assert.New(t)
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	ra := kind.CreateInstance(spec)
	if prev == nil {
		ra.Init()
	} else {
		ra.Inherit(prev)
	}

	ctx := context.New(nil)
	resp, err := httpprot.NewResponse(nil)
	assert.Nil(err)
	ctx.SetInputResponse(resp)

	resp.Std().Header.Add("X-Del", "deleted")

	ra.Handle(ctx)
	assert.Equal("mockedHeaderValue", resp.Std().Header.Get("X-Mock"))
	assert.Equal("", resp.Std().Header.Get("X-Del"))
	assert.Equal("setHeaderValue", resp.Std().Header.Get("X-Set"))

	body, err := io.ReadAll(resp.GetPayload())
	assert.Nil(err)
	assert.Equal("copyright", string(body))

	ra.Status()
	return ra.(*ResponseAdaptor)
}

func getCtx(t *testing.T, r *http.Response) *context.Context {
	ctx := context.New(tracing.NoopSpan)
	resp, err := httpprot.NewResponse(r)
	require.Nil(t, err)
	resp.FetchPayload(1024 * 1024)
	ctx.SetInputResponse(resp)
	return ctx
}

func TestCompressDecompress(t *testing.T) {
	assert := assert.New(t)
	{
		// invalid decompress parameter
		spec := &Spec{
			Decompress: "invalid",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		assert.Panics(func() { ra.Init() })
	}

	{
		// invalid compress parameter
		spec := &Spec{
			Compress: "invalid",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		assert.Panics(func() { ra.Init() })
	}
	{
		// both set compress and decompress parameter
		spec := &Spec{
			Decompress: "gzip",
			Compress:   "gzip",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		assert.Panics(func() { ra.Init() })
	}

	{
		// test compress
		spec := &Spec{
			Compress: "gzip",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		ra.Init()

		w := httptest.NewRecorder()
		_, err := w.WriteString("hello")
		assert.Nil(err)
		resp := w.Result()
		ctx := getCtx(t, resp)
		ra.Handle(ctx)
		zr, err := readers.NewGZipDecompressReader(ctx.GetInputResponse().GetPayload())
		assert.Nil(err)
		data, err := io.ReadAll(zr)
		zr.Close()
		assert.Nil(err)
		assert.Equal("hello", string(data))
	}
	{
		// test decompress
		spec := &Spec{
			Decompress: "gzip",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		ra.Init()

		// set compressed data
		w := httptest.NewRecorder()
		zr := readers.NewGZipCompressReader(strings.NewReader("hello"))
		data, err := io.ReadAll(zr)
		assert.Nil(err)
		zr.Close()
		_, err = w.Write(data)
		assert.Nil(err)
		resp := w.Result()
		resp.Header.Set(keyContentEncoding, "gzip")

		ctx := getCtx(t, resp)
		ra.Handle(ctx)
		assert.Equal("hello", string(ctx.GetInputResponse().RawPayload()))
	}
	{
		// test decompress fail
		spec := &Spec{
			Decompress: "gzip",
		}
		ra := &ResponseAdaptor{
			spec: spec,
		}
		ra.Init()

		w := httptest.NewRecorder()
		_, err := w.WriteString("hello")
		assert.Nil(err)
		resp := w.Result()
		resp.Header.Set(keyContentEncoding, "gzip")

		ctx := getCtx(t, resp)
		res := ra.Handle(ctx)
		assert.Equal(resultDecompressFailed, res)
	}
}

func TestTemplate(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `template: |
      header:
        add:
          X-Add: add-template-value
      body: hello
`
	tempSpec := &Spec{}
	codectool.MustUnmarshal([]byte(yamlConfig), tempSpec)
	spec := &Spec{
		Header: &httpheader.AdaptSpec{
			Add: map[string]string{"X-Mock": "mockedHeaderValue"},
		},
		Compress: "gzip",
		Spec:     builder.Spec{Template: tempSpec.Template},
	}
	ra := &ResponseAdaptor{
		spec: spec,
	}
	ra.Init()
	w := httptest.NewRecorder()
	resp := w.Result()
	//resp.Header.Set(keyContentEncoding, "gzip")
	ctx := getCtx(t, resp)

	zr := readers.NewGZipCompressReader(strings.NewReader("hello"))
	data, err := io.ReadAll(zr)
	assert.Nil(err)
	zr.Close()

	res := ra.Handle(ctx)
	assert.Equal("", res)
	assert.Equal("add-template-value", ctx.GetOutputResponse().Header().Get("X-Add"))
	assert.Equal("", ctx.GetOutputResponse().Header().Get("X-Mock"))
	assert.Equal(data, ctx.GetOutputResponse().RawPayload())
}
