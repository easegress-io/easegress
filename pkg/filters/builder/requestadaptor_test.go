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
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/codectool"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/v2/pkg/util/pathadaptor"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func setRequest(t *testing.T, ctx *context.Context, ns string, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	err = req.FetchPayload(1024 * 1024)
	assert.Nil(t, err)
	ctx.SetRequest(ns, req)
}

func defaultFilterSpec(spec *RequestAdaptorSpec) filters.Spec {
	spec.BaseSpec.MetaSpec.Kind = RequestAdaptorKind
	spec.BaseSpec.MetaSpec.Name = "request-adaptor"
	result, _ := filters.NewSpec(nil, "pipeline-demo", spec)
	return result
}

func getGzipEncoding(t *testing.T, data []byte) io.Reader {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	_, err := gw.Write(data)
	assert.Nil(t, err)
	return &buf
}

func TestRequestAdaptor(t *testing.T) {
	assert := assert.New(t)
	{
		// normal case
		spec := defaultFilterSpec(&RequestAdaptorSpec{})
		ra := requestAdaptorKind.CreateInstance(spec)
		ra.Init()
		assert.Equal(RequestAdaptorKind, ra.Kind().Name)
		assert.Nil(ra.Status())
		assert.Equal(spec.Name(), ra.Name())
		assert.Equal(spec, ra.Spec())

		newRA := requestAdaptorKind.CreateInstance(spec)
		newRA.Inherit(ra)
		newRA.Close()
	}

	{
		// invalid compress type
		spec := defaultFilterSpec(&RequestAdaptorSpec{Compress: "zip"})
		assert.Nil(spec)
	}

	{
		// invalid decompress type
		spec := defaultFilterSpec(&RequestAdaptorSpec{Decompress: "zip"})
		assert.Nil(spec)
	}

	{
		// compress and decompress are set together
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			Decompress: "gzip",
			Compress:   "gzip",
		})
		assert.Nil(spec)
	}

	{
		// set body and Decompress
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			Decompress: "gzip",
			RequestAdaptorTemplate: RequestAdaptorTemplate{
				Body: "body",
			},
		})
		assert.Nil(spec)
	}

	{
		// unknown API provider
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			Sign: &SignerSpec{
				APIProvider: "aws3",
			},
		})
		assert.Nil(spec)
	}
}

func TestDecompress(t *testing.T) {
	assert := assert.New(t)

	{
		// decompress without body in spec
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			Decompress: "gzip",
		})

		ra := requestAdaptorKind.CreateInstance(spec)
		ra.Init()

		{
			// compress success
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", getGzipEncoding(t, []byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := context.New(nil)
			setRequest(t, ctx, "DEFAULT", req)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)

			encoding := ctx.GetInputRequest().Header().Get("Content-Encoding")
			assert.Equal("", encoding)

			body, err := io.ReadAll(ctx.GetInputRequest().GetPayload())
			assert.Nil(err)
			assert.Equal("123", string(body))
		}

		{
			// compress fail
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader([]byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := context.New(nil)
			setRequest(t, ctx, "DEFAULT", req)

			ans := ra.Handle(ctx)
			assert.Equal(resultDecompressFailed, ans)
			ctx.Finish()
		}
	}
}

func TestCompress(t *testing.T) {
	assert := assert.New(t)

	{
		// compress without body in spec
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			Compress: "gzip",
		})
		ra := requestAdaptorKind.CreateInstance(spec)
		ra.Init()

		data := "123"
		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader([]byte(data)))
		assert.Nil(err)

		ctx := context.New(nil)
		setRequest(t, ctx, "DEFAULT", req)

		ans := ra.Handle(ctx)
		assert.Equal("", ans)

		encoding := ctx.GetInputRequest().Header().Get("Content-Encoding")
		assert.Equal("gzip", encoding)

		reader, err := gzip.NewReader(ctx.GetInputRequest().GetPayload())
		assert.Nil(err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		assert.Nil(err)
		assert.Equal("123", string(body))
	}

	{
		// compress with body in spec
		spec := defaultFilterSpec(&RequestAdaptorSpec{
			RequestAdaptorTemplate: RequestAdaptorTemplate{
				Body: "spec_body",
			},
			Compress: "gzip",
		})
		ra := requestAdaptorKind.CreateInstance(spec)
		ra.Init()

		{
			// original gzip body
			data := "123"
			req, err := http.NewRequest(http.MethodPost, "127.0.0.1", getGzipEncoding(t, []byte(data)))
			assert.Nil(err)
			req.Header.Add("Content-Encoding", "gzip")

			ctx := context.New(nil)
			setRequest(t, ctx, "DEFAULT", req)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)

			encoding := ctx.GetInputRequest().Header().Get("Content-Encoding")
			assert.Equal("gzip", encoding)

			reader, err := gzip.NewReader(ctx.GetInputRequest().GetPayload())
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

			ctx := context.New(nil)
			setRequest(t, ctx, "DEFAULT", req)

			ans := ra.Handle(ctx)
			assert.Equal("", ans)
			ctx.Finish()

			encoding := ctx.GetInputRequest().Header().Get("Content-Encoding")
			assert.Equal("gzip", encoding)

			reader, err := gzip.NewReader(ctx.GetInputRequest().GetPayload())
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

	requestAdaptorSpec := &RequestAdaptorSpec{
		RequestAdaptorTemplate: RequestAdaptorTemplate{
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
		},
		Sign: &SignerSpec{APIProvider: "aws4"},
	}
	spec := defaultFilterSpec(requestAdaptorSpec)
	ra := requestAdaptorKind.CreateInstance(spec)
	ra.Init()

	req, err := http.NewRequest(http.MethodPost, "127.0.0.1", nil)
	assert.Nil(err)

	ctx := context.New(nil)
	setRequest(t, ctx, "DEFAULT", req)

	ans := ra.Handle(ctx)
	assert.Equal("", ans)
	ctx.Finish()

	httpreq := ctx.GetInputRequest().(*httpprot.Request)
	method := httpreq.Method()
	assert.Equal(http.MethodDelete, method)

	host := httpreq.Host()
	assert.Equal("127.0.0.2", host)

	body, err := io.ReadAll(httpreq.GetPayload())
	assert.Nil(err)
	assert.Equal("123", string(body))

	headerValue := httpreq.Std().Header.Get("X-Add")
	assert.Equal("add-value", headerValue)

	headerValue = httpreq.Std().Header.Get("X-Set")
	assert.Equal("set-value", headerValue)

	path := httpreq.Path()
	assert.Equal("/path", path)

	assert.Contains(req.Header.Get("Authorization"), " SignedHeaders=host;x-add;x-amz-date;x-set,")
}

func TestRequestAdaptorTemplate(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `template: |
      header:
        add:
          X-Add: add-template-value
`
	templateSpec := &RequestAdaptorSpec{}
	codectool.MustUnmarshal([]byte(yamlConfig), templateSpec)
	spec := defaultFilterSpec(&RequestAdaptorSpec{
		RequestAdaptorTemplate: RequestAdaptorTemplate{
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
		},
		Sign: &SignerSpec{APIProvider: "aws4"},
		Spec: Spec{Template: templateSpec.Template},
	})
	ra := requestAdaptorKind.CreateInstance(spec)
	ra.Init()

	req, err := http.NewRequest(http.MethodPost, "127.0.0.1", nil)
	assert.Nil(err)

	ctx := context.New(nil)
	setRequest(t, ctx, "DEFAULT", req)

	ans := ra.Handle(ctx)
	assert.Equal("", ans)
	ctx.Finish()

	httpreq := ctx.GetInputRequest().(*httpprot.Request)
	method := httpreq.Method()
	assert.Equal(http.MethodDelete, method)

	host := httpreq.Host()
	assert.Equal("127.0.0.2", host)

	body, err := io.ReadAll(httpreq.GetPayload())
	assert.Nil(err)
	assert.Equal("123", string(body))

	headerValue := httpreq.Std().Header.Get("X-Add")
	assert.Equal("add-template-value", headerValue)

	headerValue = httpreq.Std().Header.Get("X-Set")
	assert.Equal("", headerValue)

	path := httpreq.Path()
	assert.Equal("/path", path)

	assert.Contains(req.Header.Get("Authorization"), " SignedHeaders=host;x-add;x-amz-date,")
}

func TestRequestAdaptorProcessCompressDecompress(t *testing.T) {
	assert := assert.New(t)

	ra := &RequestAdaptor{spec: &RequestAdaptorSpec{Decompress: "gzip"}}
	req, _ := httpprot.NewRequest(nil)

	assert.Empty(ra.processCompress(req))
	assert.Equal("gzip", req.HTTPHeader().Get(keyContentEncoding))

	req.HTTPHeader().Del("Content-Encoding")
	req.SetPayload(bytes.NewReader([]byte("123")))
	assert.Empty(ra.processCompress(req))
	assert.Equal("gzip", req.HTTPHeader().Get(keyContentEncoding))
	assert.EqualValues(-1, req.ContentLength)

	assert.Empty(ra.processCompress(req))
	assert.Empty(ra.processDecompress(req))
	assert.Empty(req.HTTPHeader().Get(keyContentEncoding))

	assert.Empty(ra.processDecompress(req))
}
