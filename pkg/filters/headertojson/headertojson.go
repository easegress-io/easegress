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

// Package headertojson implements a filter to convert HTTP request header to json.
package headertojson

import (
	"errors"
	"io"
	"net/http"

	json "github.com/goccy/go-json"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
)

const (
	// Kind is the kind of Kafka
	Kind = "HeaderToJSON"

	resultJSONEncodeDecodeErr = "jsonEncodeDecodeErr"
	resultBodyReadErr         = "bodyReadErr"
)

var (
	errJSONEncodeDecode = errors.New(resultJSONEncodeDecodeErr)
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "HeaderToJSON convert http request header to json",
	Results: []string{
		resultJSONEncodeDecodeErr,
		resultBodyReadErr,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HeaderToJSON{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// HeaderToJSON put http request headers into body as JSON fields.
	HeaderToJSON struct {
		spec      *Spec
		headerMap map[string]string
	}
)

var _ filters.Filter = (*HeaderToJSON)(nil)

// Name returns the name of the HeaderToJSON filter instance.
func (h *HeaderToJSON) Name() string {
	return h.spec.Name()
}

// Kind return kind of HeaderToJSON
func (h *HeaderToJSON) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the HeaderToJSON
func (h *HeaderToJSON) Spec() filters.Spec {
	return h.spec
}

func (h *HeaderToJSON) init() {
	h.headerMap = make(map[string]string)
	for _, header := range h.spec.HeaderMap {
		h.headerMap[http.CanonicalHeaderKey(header.Header)] = header.JSON
	}
}

// Init init HeaderToJSON
func (h *HeaderToJSON) Init() {
	h.init()
}

// Inherit init HeaderToJSON based on previous generation
func (h *HeaderToJSON) Inherit(previousGeneration filters.Filter) {
	h.Init()
}

// Close close HeaderToJSON
func (h *HeaderToJSON) Close() {
}

// Status return status of HeaderToJSON
func (h *HeaderToJSON) Status() interface{} {
	return nil
}

// Handle handle Context
func (h *HeaderToJSON) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest()
	headerMap := make(map[string]interface{})
	for header, json := range h.headerMap {
		value := req.Header().Get(header)
		if value != "" {
			headerMap[json] = value
		}
	}
	if len(headerMap) == 0 {
		return ""
	}

	if req.IsStream() {
		return resultBodyReadErr
	}

	reqBody := req.RawPayload()

	var body interface{}
	if len(reqBody) == 0 {
		body = headerMap
	} else {
		var err error
		if body, err = getNewBody(reqBody, headerMap); err != nil {
			return resultJSONEncodeDecodeErr
		}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return resultJSONEncodeDecodeErr
	}

	req.SetPayload(bodyBytes)
	return ""
}

func decodeMapJSON(body []byte) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	err := json.Unmarshal(body, &res)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return res, nil
}

func decodeArrayJSON(body []byte) ([]map[string]interface{}, error) {
	res := []map[string]interface{}{}
	err := json.Unmarshal(body, &res)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return res, nil
}

func mergeMap(m1 map[string]interface{}, m2 map[string]interface{}) map[string]interface{} {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func mergeMapToArrayMap(arrayMap []map[string]interface{}, m map[string]interface{}) []map[string]interface{} {
	for i := range arrayMap {
		arrayMap[i] = mergeMap(arrayMap[i], m)
	}
	return arrayMap
}

func firstNonBlandByte(bytes []byte) byte {
	for _, b := range bytes {
		switch b {
		case '\n', '\t', '\r', ' ':
			continue
		}
		return b
	}
	return 0
}

func getNewBody(reqBody []byte, headerMap map[string]interface{}) (interface{}, error) {
	char := firstNonBlandByte(reqBody)

	if char == '{' {
		bodyMap, err := decodeMapJSON(reqBody)
		if err != nil {
			return nil, errJSONEncodeDecode
		}
		return mergeMap(bodyMap, headerMap), nil

	} else if char == '[' {
		bodyArray, err := decodeArrayJSON(reqBody)
		if err != nil {
			return nil, errJSONEncodeDecode
		}
		return mergeMapToArrayMap(bodyArray, headerMap), nil
	}
	return nil, errJSONEncodeDecode
}
