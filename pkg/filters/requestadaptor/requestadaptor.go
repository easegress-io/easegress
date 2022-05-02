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

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"

	resultReadBodyFail   = "readBodyFail"
	resultDecompressFail = "decompressFail"
	resultCompressFail   = "compressFail"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "RequestAdaptor adapts request.",
	Results:     []string{resultDecompressFail, resultCompressFail, resultReadBodyFail},
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
	previousGeneration.Close()
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
	httpreq := ctx.Request().(*httpprot.Request)
	method, path, _ := httpreq.Method(), httpreq.Path(), httpreq.Header()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ",
			method, " adapted to ", ra.spec.Method))
		httpreq.SetMethod(ra.spec.Method)
	}

	if ra.pa != nil {
		adaptedPath := ra.pa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ",
				path, " adapted to ", adaptedPath))
		}
		httpreq.SetPath(adaptedPath)
	}

	if ra.spec.Header != nil {
		adaptHeader(httpreq, ra.spec.Header)
	}

	if len(ra.spec.Body) != 0 {
		httpreq.SetPayload([]byte(ra.spec.Body))
		httpreq.Std().Header.Del("Content-Encoding")
	}

	if len(ra.spec.Host) != 0 {
		httpreq.SetHost(ra.spec.Host)
	}

	if ra.spec.Compress != "" {
		res := ra.processCompress(ctx)
		if res != "" {
			return res
		}
	}

	if ra.spec.Decompress != "" {
		res := ra.processDecompress(ctx)
		if res != "" {
			return res
		}
	}
	return ""
}

func (ra *RequestAdaptor) processCompress(ctx *context.Context) string {
	encoding := ctx.Request().Header().Get("Content-Encoding")
	if encoding != "" {
		return ""
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := io.Copy(gw, ctx.Request().GetPayload())
	if err != nil {
		logger.Errorf("compress request body failed, %v", err)
		return resultCompressFail
	}
	gw.Close()

	ctx.Request().SetPayload(buf.Bytes())
	ctx.Request().Header().Set("Content-Encoding", "gzip")
	return ""
}

func (ra *RequestAdaptor) processDecompress(ctx *context.Context) string {
	encoding := ctx.Request().Header().Get("Content-Encoding")
	if ra.spec.Decompress == "gzip" && encoding == "gzip" {
		reader, err := gzip.NewReader(ctx.Request().GetPayload())
		if err != nil {
			return resultDecompressFail
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			return resultDecompressFail
		}
		ctx.Request().SetPayload(data)
		ctx.Request().Header().Del("Content-Encoding")
	}
	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} { return nil }

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {}
