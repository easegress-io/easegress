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

package headertojson

import (
	"bytes"
	"io"
	"net/http"

	json "github.com/goccy/go-json"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "HeaderToJSON"

	resultJSONEncodeDecodeErr = "JSONEncodeDecodeErr"
)

func init() {
	httppipeline.Register(&HeaderToJSON{})
}

type (
	// HeaderToJSON put http request headers into body as JSON fields.
	HeaderToJSON struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
		headerMap  map[string]string
	}
)

var _ httppipeline.Filter = (*HeaderToJSON)(nil)

// Kind return kind of HeaderToJSON
func (h *HeaderToJSON) Kind() string {
	return Kind
}

// DefaultSpec return default spec of HeaderToJSON
func (h *HeaderToJSON) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of HeaderToJSON
func (h *HeaderToJSON) Description() string {
	return "HeaderToJSON convert http request header to json"
}

// Results return possible results of HeaderToJSON
func (h *HeaderToJSON) Results() []string {
	return []string{resultJSONEncodeDecodeErr}
}

func (h *HeaderToJSON) init() {
	h.headerMap = make(map[string]string)
	for _, header := range h.spec.HeaderMap {
		h.headerMap[http.CanonicalHeaderKey(header.Header)] = header.JSON
	}
}

// Init init HeaderToJSON
func (h *HeaderToJSON) Init(filterSpec *httppipeline.FilterSpec) {
	h.filterSpec, h.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	h.init()
}

// Inherit init HeaderToJSON based on previous generation
func (h *HeaderToJSON) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	h.Init(filterSpec)
}

// Close close HeaderToJSON
func (h *HeaderToJSON) Close() {
}

// Status return status of HeaderToJSON
func (h *HeaderToJSON) Status() interface{} {
	return nil
}

func (h *HeaderToJSON) encodeJSON(input map[string]interface{}) ([]byte, error) {
	return json.Marshal(input)
}

func (h *HeaderToJSON) decodeJSON(ctx context.HTTPContext) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	decoder := json.NewDecoder(ctx.Request().Body())
	err := decoder.Decode(&res)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return res, nil
}

// Handle handle HTTPContext
func (h *HeaderToJSON) Handle(ctx context.HTTPContext) string {
	result := h.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (h *HeaderToJSON) handle(ctx context.HTTPContext) string {
	headerMap := make(map[string]interface{})
	for header, json := range h.headerMap {
		value := ctx.Request().Header().Get(header)
		if value != "" {
			headerMap[json] = value
		}
	}
	if len(headerMap) == 0 {
		return ""
	}

	bodyMap, err := h.decodeJSON(ctx)
	if err != nil {
		return resultJSONEncodeDecodeErr
	}
	for k, v := range headerMap {
		bodyMap[k] = v
	}
	body, err := h.encodeJSON(bodyMap)
	if err != nil {
		return resultJSONEncodeDecodeErr
	}
	ctx.Request().SetBody(bytes.NewReader(body))
	return ""
}
