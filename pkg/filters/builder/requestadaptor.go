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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/v2/pkg/util/pathadaptor"
	"github.com/megaease/easegress/v2/pkg/util/readers"
	"github.com/megaease/easegress/v2/pkg/util/signer"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

const (
	// RequestAdaptorKind is the kind of RequestAdaptor.
	RequestAdaptorKind = "RequestAdaptor"

	resultDecompressFailed = "decompressFailed"
	resultCompressFailed   = "compressFailed"
	resultSignFailed       = "signFailed"

	keyContentLength   = "Content-Length"
	keyContentEncoding = "Content-Encoding"
)

var requestAdaptorKind = &filters.Kind{
	Name:        RequestAdaptorKind,
	Description: "RequestAdaptor adapts request.",
	Results: []string{
		resultDecompressFailed,
		resultCompressFailed,
	},
	DefaultSpec: func() filters.Spec {
		return &RequestAdaptorSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &RequestAdaptor{spec: spec.(*RequestAdaptorSpec)}
	},
}

var signerConfigs = map[string]signerConfig{
	"aws4": {
		literal: &signer.Literal{
			ScopeSuffix:      "aws4_request",
			AlgorithmName:    "X-Amz-Algorithm",
			AlgorithmValue:   "AWS4-HMAC-SHA256",
			SignedHeaders:    "X-Amz-SignedHeaders",
			Signature:        "X-Amz-Signature",
			Date:             "X-Amz-Date",
			Expires:          "X-Amz-Expires",
			Credential:       "X-Amz-Credential",
			ContentSHA256:    "X-Amz-Content-Sha256",
			SigningKeyPrefix: "AWS4",
		},

		headerHoisting: &signer.HeaderHoisting{
			AllowedPrefix:    []string{"X-Amz-"},
			DisallowedPrefix: []string{"X-Amz-Meta-"},
			Disallowed: []string{
				"Cache-Control",
				"Content-Disposition",
				"Content-Encoding",
				"Content-Language",
				"Content-Md5",
				"Content-Type",
				"Expires",
				"If-Match",
				"If-Modified-Since",
				"If-None-Match",
				"If-Unmodified-Since",
				"Range",
				"X-Amz-Acl",
				"X-Amz-Copy-Source",
				"X-Amz-Copy-Source-If-Match",
				"X-Amz-Copy-Source-If-Modified-Since",
				"X-Amz-Copy-Source-If-None-Match",
				"X-Amz-Copy-Source-If-Unmodified-Since",
				"X-Amz-Copy-Source-Range",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Algorithm",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key-Md5",
				"X-Amz-Grant-Full-control",
				"X-Amz-Grant-Read",
				"X-Amz-Grant-Read-Acp",
				"X-Amz-Grant-Write",
				"X-Amz-Grant-Write-Acp",
				"X-Amz-Metadata-Directive",
				"X-Amz-Mfa",
				"X-Amz-Request-Payer",
				"X-Amz-Server-Side-Encryption",
				"X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id",
				"X-Amz-Server-Side-Encryption-Customer-Algorithm",
				"X-Amz-Server-Side-Encryption-Customer-Key",
				"X-Amz-Server-Side-Encryption-Customer-Key-Md5",
				"X-Amz-Storage-Class",
				"X-Amz-Tagging",
				"X-Amz-Website-Redirect-Location",
				"X-Amz-Content-Sha256",
			},
		},
	},
}

func init() {
	filters.Register(requestAdaptorKind)
}

type (
	// RequestAdaptor is filter RequestAdaptor.
	RequestAdaptor struct {
		spec *RequestAdaptorSpec
		Builder

		pa     *pathadaptor.PathAdaptor
		signer *signer.Signer
	}

	// RequestAdaptorSpec is HTTPAdaptor RequestAdaptorSpec.
	RequestAdaptorSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`

		RequestAdaptorTemplate `json:",inline"`
		Compress               string      `json:"compress,omitempty"`
		Decompress             string      `json:"decompress,omitempty"`
		Sign                   *SignerSpec `json:"sign,omitempty"`
	}

	// RequestAdaptorTemplate is the template of the request adaptor.
	RequestAdaptorTemplate struct {
		Host   string                `json:"host,omitempty"`
		Method string                `json:"method,omitempty" jsonschema:"format=httpmethod"`
		Path   *pathadaptor.Spec     `json:"path,omitempty"`
		Header *httpheader.AdaptSpec `json:"header,omitempty"`
		Body   string                `json:"body,omitempty"`
	}

	// SignerSpec is the spec of the request signer.
	SignerSpec struct {
		signer.Spec `json:",inline"`
		APIProvider string   `json:"apiProvider,omitempty" jsonschema:"enum=,enum=aws4"`
		Scopes      []string `json:"scopes,omitempty"`
	}

	signerConfig struct {
		literal        *signer.Literal
		headerHoisting *signer.HeaderHoisting
	}
)

// Validate verifies that at least one of the validations is defined.
func (spec *RequestAdaptorSpec) Validate() error {
	if spec.Decompress != "" && spec.Decompress != "gzip" {
		return fmt.Errorf("RequestAdaptor only support decompress type of gzip")
	}
	if spec.Compress != "" && spec.Compress != "gzip" {
		return fmt.Errorf("RequestAdaptor only support decompress type of gzip")
	}
	if spec.Compress != "" && spec.Decompress != "" {
		return fmt.Errorf("RequestAdaptor can only do compress or decompress for given request body, not both")
	}
	if spec.Body != "" && spec.Decompress != "" {
		return fmt.Errorf("No need to decompress when body is specified in RequestAdaptor spec")
	}
	if spec.Sign == nil {
		return nil
	}
	s := spec.Sign
	if s.APIProvider != "" {
		if _, ok := signerConfigs[s.APIProvider]; !ok {
			return fmt.Errorf("%q is not a supported API provider", s.APIProvider)
		}
	}

	return nil
}

// Name returns the name of the RequestAdaptor filter instance.
func (ra *RequestAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of RequestAdaptor.
func (ra *RequestAdaptor) Kind() *filters.Kind {
	return requestAdaptorKind
}

// Spec returns the spec used by the RequestAdaptor
func (ra *RequestAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes RequestAdaptor.
func (ra *RequestAdaptor) Init() {
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
	if s := ra.spec.Sign; s != nil {
		sc, ok := signerConfigs[s.APIProvider]
		if ok {
			s.Literal = sc.literal
			s.HeaderHoisting = sc.headerHoisting
		}
		ra.signer = signer.CreateFromSpec(&s.Spec)
	}
	if ra.spec.Template != "" {
		ra.Builder.reload(&ra.spec.Spec)
	}
}

func adaptHeader(h http.Header, as *httpheader.AdaptSpec) {
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
	req := ctx.GetInputRequest().(*httpprot.Request)
	method, path := req.Method(), req.Path()

	templateSpec := &RequestAdaptorTemplate{}
	if ra.spec.Template != "" {
		data, err := prepareBuilderData(ctx)
		if err != nil {
			logger.Warnf("prepareBuilderData failed: %v", err)
			return resultBuildErr
		}
		if err = ra.Builder.build(data, templateSpec); err != nil {
			msgFmt := "RequestAdaptor(%s): failed to build adaptor info: %v"
			logger.Warnf(msgFmt, ra.Name(), err)
			return resultBuildErr
		}
	}
	newMethod := templateSpec.Method
	if newMethod == "" {
		newMethod = ra.spec.Method
	}
	if newMethod != "" && newMethod != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ", method, " adapted to ", newMethod))
		req.SetMethod(newMethod)
	}

	var newPa *pathadaptor.PathAdaptor
	if templateSpec.Path == nil {
		newPa = ra.pa
	} else {
		newPa = pathadaptor.New(templateSpec.Path)
	}
	if newPa != nil {
		adaptedPath := newPa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ", path, " adapted to ", adaptedPath))
		}
		req.SetPath(adaptedPath)
	}

	newHeader := templateSpec.Header
	if newHeader == nil {
		newHeader = ra.spec.Header
	}
	if newHeader != nil {
		adaptHeader(req.Std().Header, newHeader)
	}

	newBody := templateSpec.Body
	if newBody == "" {
		newBody = ra.spec.Body
	}
	if len(newBody) != 0 {
		req.SetPayload([]byte(newBody))
		req.Std().Header.Del("Content-Encoding")
	}

	newHost := templateSpec.Host
	if newHost == "" {
		newHost = ra.spec.Host
	}
	if len(newHost) != 0 {
		req.SetHost(newHost)
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

	if ra.signer != nil {
		res := ra.signRequest(req)
		if res != "" {
			return res
		}
	}

	return ""
}

func (ra *RequestAdaptor) processCompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get(keyContentEncoding)
	if encoding != "" {
		return ""
	}

	zr := readers.NewGZipCompressReader(req.GetPayload())
	if req.IsStream() {
		req.SetPayload(zr)
		req.ContentLength = -1
		req.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("compress request body failed: %v", err)
			return resultCompressFailed
		}
		req.SetPayload(data)
		req.ContentLength = int64(len(data))
		req.HTTPHeader().Set(keyContentLength, strconv.Itoa(len(data)))
	}

	req.HTTPHeader().Set(keyContentEncoding, "gzip")
	return ""
}

func (ra *RequestAdaptor) processDecompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get(keyContentEncoding)
	if ra.spec.Decompress != "gzip" || encoding != "gzip" {
		return ""
	}

	zr, err := readers.NewGZipDecompressReader(req.GetPayload())
	if err != nil {
		return resultDecompressFailed
	}

	if req.IsStream() {
		req.SetPayload(zr)
		req.ContentLength = -1
		req.HTTPHeader().Del(keyContentLength)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("decompress request body failed: %v", err)
			return resultDecompressFailed
		}
		req.SetPayload(data)
		req.ContentLength = int64(len(data))
		req.HTTPHeader().Set(keyContentLength, strconv.Itoa(len(data)))
	}

	req.HTTPHeader().Del(keyContentEncoding)
	return ""
}

func (ra *RequestAdaptor) signRequest(req *httpprot.Request) string {
	sCtx := ra.signer.NewSigningContext(time.Now(), ra.spec.Sign.Scopes...)
	if req.IsStream() {
		sCtx.ExcludeBody(true)
	}
	err := sCtx.Sign(req.Std(), req.GetPayload)
	if err != nil {
		logger.Errorf("sign request failed: %v", err)
		return resultSignFailed
	}
	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} {
	return nil
}

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {
}
