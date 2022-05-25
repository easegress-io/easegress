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
	"io"
	"strconv"
	"strings"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/readers"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"

	resultResponseNotFound = "responseNotFound"
	resultDecompressFailed = "decompressFailed"
	resultCompressFailed   = "compressFailed"

	keyContentLength   = "Content-Length"
	keyContentEncoding = "Content-Encoding"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "ResponseAdaptor adapts response.",
	Results: []string{
		resultResponseNotFound,
		resultCompressFailed,
		resultDecompressFailed,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ResponseAdaptor{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// ResponseAdaptor is filter ResponseAdaptor.
	ResponseAdaptor struct {
		spec *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Header     *httpheader.AdaptSpec `yaml:"header" jsonschema:"omitempty"`
		Body       string                `yaml:"body" jsonschema:"omitempty"`
		Compress   string                `yaml:"compress" jsonschema:"omitempty"`
		Decompress string                `yaml:"decompress" jsonschema:"omitempty"`
	}
)

// Name returns the name of the ResponseAdaptor filter instance.
func (ra *ResponseAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of ResponseAdaptor.
func (ra *ResponseAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the ResponseAdaptor
func (ra *ResponseAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes ResponseAdaptor.
func (ra *ResponseAdaptor) Init() {
	if ra.spec.Decompress != "" && ra.spec.Decompress != "gzip" {
		panic("ResponseAdaptor only support decompress type of gzip")
	}
	if ra.spec.Compress != "" && ra.spec.Compress != "gzip" {
		panic("ResponseAdaptor only support decompress type of gzip")
	}
	if ra.spec.Compress != "" && ra.spec.Decompress != "" {
		panic("ResponseAdaptor can only do compress or decompress for given request body, not both")
	}
	if ra.spec.Body != "" && ra.spec.Decompress != "" {
		panic("No need to decompress when body is specified in ResponseAdaptor spec")
	}
	ra.reload()
}

// Inherit inherits previous generation of ResponseAdaptor.
func (ra *ResponseAdaptor) Inherit(previousGeneration filters.Filter) {
	ra.reload()
}

func (ra *ResponseAdaptor) reload() {
	// Nothing to do.
}

func adaptHeader(req *httpprot.Response, as *httpheader.AdaptSpec) {
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

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx *context.Context) string {
	resp := ctx.GetResponse(ctx.TargetResponseID())
	if resp == nil {
		return resultResponseNotFound
	}
	egresp := resp.(*httpprot.Response)

	if ra.spec.Header != nil {
		adaptHeader(egresp, ra.spec.Header)
	}

	if len(ra.spec.Body) != 0 {
		egresp.SetPayload([]byte(ra.spec.Body))
		egresp.HTTPHeader().Del("Content-Encoding")
	}

	if ra.spec.Compress != "" {
		if res := ra.compress(egresp); res != "" {
			return res
		}
	}

	if ra.spec.Decompress != "" {
		if res := ra.decompress(egresp); res != "" {
			return res
		}
	}

	return ""
}

func (ra *ResponseAdaptor) compress(resp *httpprot.Response) string {
	for _, ce := range resp.HTTPHeader().Values(keyContentEncoding) {
		if strings.Contains(ce, "gzip") {
			return ""
		}
	}

	zr := readers.NewGZipCompressReader(resp.GetPayload())
	if resp.IsStream() {
		resp.SetPayload(zr)
		resp.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("compress response body failed, %v", err)
			return resultCompressFailed
		}
		resp.SetPayload(data)
		resp.HTTPHeader().Set(keyContentLength, strconv.Itoa(len(data)))
	}

	resp.HTTPHeader().Set(keyContentEncoding, "gzip")
	return ""
}

func (ra *ResponseAdaptor) decompress(resp *httpprot.Response) string {
	if ra.spec.Decompress != "gzip" {
		return ""
	}

	encoding := resp.HTTPHeader().Get(keyContentEncoding)
	if encoding != "gzip" {
		return ""
	}

	zr, err := readers.NewGZipDecompressReader(resp.GetPayload())
	if err != nil {
		return resultDecompressFailed
	}

	if resp.IsStream() {
		resp.SetPayload(zr)
		resp.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("decompress response body failed, %v", err)
			return resultDecompressFailed
		}
		resp.SetPayload(data)
		resp.HTTPHeader().Set(keyContentLength, strconv.Itoa(len(data)))
	}

	resp.HTTPHeader().Del(keyContentEncoding)
	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} {
	return nil
}

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {
}
