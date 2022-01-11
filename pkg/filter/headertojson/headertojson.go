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
	Kind = "HeaderToJson"

	resultJsonEncodeDecodeErr = "jsonEncodeDecodeErr"
)

func init() {
	httppipeline.Register(&HeaderToJson{})
}

type (
	// HeaderToJson is make http request header to json
	HeaderToJson struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
		headerMap  map[string]string
	}
)

var _ httppipeline.Filter = (*HeaderToJson)(nil)

// Kind return kind of HeaderToJson
func (h *HeaderToJson) Kind() string {
	return Kind
}

// DefaultSpec return default spec of HeaderToJson
func (h *HeaderToJson) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of HeaderToJson
func (h *HeaderToJson) Description() string {
	return "HeaderToJson convert http request header to json"
}

// Results return possible results of HeaderToJson
func (h *HeaderToJson) Results() []string {
	return []string{resultJsonEncodeDecodeErr}
}

func (h *HeaderToJson) init() {
	h.headerMap = make(map[string]string)
	for _, header := range h.spec.HeaderMap {
		h.headerMap[header.Header] = header.Json
	}
}

// Init init HeaderToJson
func (h *HeaderToJson) Init(filterSpec *httppipeline.FilterSpec) {
	h.filterSpec, h.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	h.init()
}

// Inherit init HeaderToJson based on previous generation
func (h *HeaderToJson) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	h.Init(filterSpec)
}

// Close close HeaderToJson
func (h *HeaderToJson) Close() {
}

// Status return status of HeaderToJson
func (h *HeaderToJson) Status() interface{} {
	return nil
}

func (h *HeaderToJson) encodeJson(input map[string]interface{}) ([]byte, error) {
	return json.Marshal(input)
}

func (h *HeaderToJson) decodeJson(ctx context.HTTPContext) (map[string]interface{}, error) {
	ans := make(map[string]interface{})

	b, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return nil, err
	}
	json.Unmarshal(b, &ans)
	return ans, nil
}

// HandleMQTT handle MQTT context
func (h *HeaderToJson) Handle(ctx context.HTTPContext) (result string) {
	ans, err := h.decodeJson(ctx)
	if err != nil {
		return resultJsonEncodeDecodeErr
	}
	for header, json := range h.headerMap {
		value := ctx.Request().Header().Get(http.CanonicalHeaderKey(header))
		ans[json] = value
	}
	body, err := h.encodeJson(ans)
	if err != nil {
		return resultJsonEncodeDecodeErr
	}
	ctx.Request().SetBody(bytes.NewReader(body))
	return ""
}
