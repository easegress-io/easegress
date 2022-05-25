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
	"io"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"

	resultReadBodyFailed   = "readBodyFailed"
	resultDecompressFailed = "decompressFailed"
	resultCompressFailed   = "compressFailed"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "RequestAdaptor adapts request.",
	Results: []string{
		resultDecompressFailed,
		resultCompressFailed,
		resultReadBodyFailed,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &RequestAdaptor{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// RequestAdaptor is filter RequestAdaptor.
	RequestAdaptor struct {
		spec *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Host       string                `yaml:"host" jsonschema:"omitempty"`
		Method     string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path       *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header     *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		Body       string                `yaml:"body" jsonschema:"omitempty"`
		Compress   string                `yaml:"compress" jsonschema:"omitempty"`
		Decompress string                `yaml:"decompress" jsonschema:"omitempty"`
	}
)

// Name returns the name of the RequestAdaptor filter instance.
func (ra *RequestAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of RequestAdaptor.
func (ra *RequestAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the RequestAdaptor
func (ra *RequestAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes RequestAdaptor.
func (ra *RequestAdaptor) Init() {
	if ra.spec.Decompress != "" && ra.spec.Decompress != "gzip" {
		panic("RequestAdaptor only support decompress type of gzip")
	}
	if ra.spec.Compress != "" && ra.spec.Compress != "gzip" {
		panic("RequestAdaptor only support decompress type of gzip")
	}
	if ra.spec.Compress != "" && ra.spec.Decompress != "" {
		panic("RequestAdaptor can only do compress or decompress for given request body, not both")
	}
	if ra.spec.Body != "" && ra.spec.Decompress != "" {
		panic("No need to decompress when body is specified in RequestAdaptor spec")
	}
	ra.reload()
}

// Inherit inherits previous generation of RequestAdaptor.
func (ra *RequestAdaptor) Inherit(previousGeneration filters.Filter) {
	ra.Init()
}

func (ra *RequestAdaptor) reload() {
	if ra.spec.Path != nil {
		ra.pa = pathadaptor.New(ra.spec.Path)
	}
}

func adaptHeader(req *httpprot.Request, as *httpheader.AdaptSpec) {
	h := req.Std().Header
	for _, key := range as.Del {
		h.Del(key)
	}
	for key, value := range as.Set {
		h.Set(key, value)
	}
	for key, value := range as.Add {
		h.Add(key, value)
	}
}

// Handle adapts request.
func (ra *RequestAdaptor) Handle(ctx *context.Context) string {
	req := ctx.Request().(*httpprot.Request)
	method, path := req.Method(), req.Path()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ", method, " adapted to ", ra.spec.Method))
		req.SetMethod(ra.spec.Method)
	}

	if ra.pa != nil {
		adaptedPath := ra.pa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ", path, " adapted to ", adaptedPath))
		}
		req.SetPath(adaptedPath)
	}

	if ra.spec.Header != nil {
		adaptHeader(req, ra.spec.Header)
	}

	if len(ra.spec.Body) != 0 {
		req.SetPayload([]byte(ra.spec.Body))
		req.Std().Header.Del("Content-Encoding")
	}

	if len(ra.spec.Host) != 0 {
		req.SetHost(ra.spec.Host)
	}

	if ra.spec.Compress != "" {
		res := ra.processCompress(req)
		if res != "" {
			return res
		}
	}

	if ra.spec.Decompress != "" {
		res := ra.processDecompress(req)
		if res != "" {
			return res
		}
	}

	return ""
}

func (ra *RequestAdaptor) processCompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get("Content-Encoding")
	if encoding != "" {
		return ""
	}

	zr := readers.NewGZipCompressReader(req.GetPayload())
	if req.IsStream() {
		req.SetPayload(zr)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("compress request body failed, %v", err)
			return resultCompressFailed
		}
		req.SetPayload(data)
	}

	req.HTTPHeader().Set("Content-Encoding", "gzip")
	return ""
}

func (ra *RequestAdaptor) processDecompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get("Content-Encoding")
	if ra.spec.Decompress != "gzip" || encoding != "gzip" {
		return ""
	}

	zr, err := readers.NewGZipDecompressReader(req.GetPayload())
	if err != nil {
		return resultDecompressFailed
	}

	if req.IsStream() {
		req.SetPayload(zr)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("decompress request body failed, %v", err)
			return resultDecompressFailed
		}
		req.SetPayload(data)
	}

	req.HTTPHeader().Del("Content-Encoding")
	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} {
	return nil
}

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {
}
