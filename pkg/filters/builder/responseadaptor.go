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
	"io"
	"strconv"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/v2/pkg/util/readers"
)

const (
	// ResponseAdaptorKind is the kind of ResponseAdaptor.
	ResponseAdaptorKind = "ResponseAdaptor"

	resultResponseNotFound = "responseNotFound"
)

var responseAdaptorKind = &filters.Kind{
	Name:        ResponseAdaptorKind,
	Description: "ResponseAdaptor adapts response.",
	Results: []string{
		resultResponseNotFound,
		resultCompressFailed,
		resultDecompressFailed,
	},
	DefaultSpec: func() filters.Spec {
		return &ResponseAdaptorSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ResponseAdaptor{spec: spec.(*ResponseAdaptorSpec)}
	},
}

func init() {
	filters.Register(responseAdaptorKind)
}

type (
	// ResponseAdaptor is filter ResponseAdaptor.
	ResponseAdaptor struct {
		spec *ResponseAdaptorSpec
		Builder
	}

	// ResponseAdaptorSpec is HTTPAdaptor ResponseAdaptorSpec.
	ResponseAdaptorSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`

		ResponseAdaptorTemplate `json:",inline"`
		Compress                string `json:"compress,omitempty"`
		Decompress              string `json:"decompress,omitempty"`
	}

	// ResponseAdaptorTemplate is the template of ResponseAdaptor.
	ResponseAdaptorTemplate struct {
		Header *httpheader.AdaptSpec `json:"header,omitempty"`
		Body   string                `json:"body,omitempty"`
	}
)

// Name returns the name of the ResponseAdaptor filter instance.
func (ra *ResponseAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of ResponseAdaptor.
func (ra *ResponseAdaptor) Kind() *filters.Kind {
	return responseAdaptorKind
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
	if ra.spec.Template != "" {
		ra.Builder.reload(&ra.spec.Spec)
	}
}

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx *context.Context) string {
	resp := ctx.GetInputResponse()
	if resp == nil {
		return resultResponseNotFound
	}
	egresp := resp.(*httpprot.Response)

	templateSpec := &ResponseAdaptorTemplate{}
	if ra.spec.Template != "" {
		data, err := prepareBuilderData(ctx)
		if err != nil {
			logger.Warnf("prepareBuilderData failed: %v", err)
			return resultBuildErr
		}

		if err = ra.Builder.build(data, templateSpec); err != nil {
			msgFmt := "ResponseAdaptor(%s): failed to build adaptor info: %v"
			logger.Warnf(msgFmt, ra.Name(), err)
			return resultBuildErr
		}
	}

	newHeader := templateSpec.Header
	if newHeader == nil {
		newHeader = ra.spec.Header
	}
	if newHeader != nil {
		adaptHeader(egresp.Std().Header, newHeader)
	}

	newBody := templateSpec.Body
	if newBody == "" {
		newBody = ra.spec.Body
	}
	if len(newBody) != 0 {
		egresp.SetPayload([]byte(newBody))
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
		resp.ContentLength = -1
		resp.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("compress response body failed, %v", err)
			return resultCompressFailed
		}
		resp.SetPayload(data)
		resp.ContentLength = int64(len(data))
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
		resp.ContentLength = -1
		resp.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("decompress response body failed, %v", err)
			return resultDecompressFailed
		}
		resp.SetPayload(data)
		resp.ContentLength = int64(len(data))
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
