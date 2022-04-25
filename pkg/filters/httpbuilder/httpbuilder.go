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

package httpbuilder

import (
	"encoding/json"
	"net/http"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"gopkg.in/yaml.v3"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// HTTPBuilder is the base HTTP builder.
	HTTPBuilder struct {
		bodyBuilder    *builder
		headerBuilders map[*builder][]*builder
	}

	// Spec is the spec of HTTPBuilder.
	Spec struct {
		Headers map[string][]string `yaml:"headers" jsonschema:"omitempty"`
		Body    string              `yaml:"body" jsonschema:"omitempty"`
	}

	builderData struct {
		Requests  map[string]*request
		Responses map[string]*response
	}

	request struct {
		*http.Request
		rawBody    []byte
		parsedBody interface{}
	}

	response struct {
		*http.Response
		rawBody    []byte
		parsedBody interface{}
	}
)

func (b *HTTPBuilder) reload(spec *Spec) {
	b.bodyBuilder = newBuilder(spec.Body)

	b.headerBuilders = map[*builder][]*builder{}
	for key, values := range spec.Headers {
		kb := newBuilder(key)
		vbs := make([]*builder, 0, len(values))
		for _, v := range values {
			vbs = append(vbs, newBuilder(v))
		}
		b.headerBuilders[kb] = vbs
	}
}

func (b *HTTPBuilder) buildBody(data *builderData) ([]byte, error) {
	if b.bodyBuilder == nil {
		return nil, nil
	}

	body, err := b.bodyBuilder.build(data)
	if err != nil {
		logger.Warnf("build body failed: %v", err)
		return nil, err
	}

	return body, nil
}

func (b *HTTPBuilder) buildHeader(data *builderData) (http.Header, error) {
	h := http.Header{}

	for kb, vbs := range b.headerBuilders {
		key, err := kb.buildString(data)
		if err != nil {
			logger.Warnf("build header key failed: %v", err)
			return nil, err
		}
		for i, vb := range vbs {
			value, err := vb.buildString(data)
			if err != nil {
				logger.Warnf("build header value %d failed: %v", i, err)
				return nil, err
			}
			h.Add(key, value)
		}
	}

	return h, nil
}

// Status returns status.
func (b *HTTPBuilder) Status() interface{} {
	return nil
}

// Close closes HTTPBuilder.
func (b *HTTPBuilder) Close() {
}

// RawBody returns the body as raw bytes.
func (r *request) RawBody() []byte {
	return r.rawBody
}

// Body returns the body as a string.
func (r *request) Body() string {
	return string(r.rawBody)
}

// JSONBody parses the body as a JSON object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *request) JSONBody() (interface{}, error) {
	if r.parsedBody == nil {
		var v interface{}
		err := json.Unmarshal(r.rawBody, &v)
		if err != nil {
			return nil, err
		}
		r.parsedBody = v
	}
	return r.parsedBody, nil
}

// YAMLBody parses the body as a YAML object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *request) YAMLBody() (interface{}, error) {
	if r.parsedBody == nil {
		var v interface{}
		err := yaml.Unmarshal(r.rawBody, &v)
		if err != nil {
			return nil, err
		}
		r.parsedBody = v
	}
	return r.parsedBody, nil
}

// RawBody returns the body as raw bytes.
func (r *response) RawBody() []byte {
	return r.rawBody
}

// Body returns the body as a string.
func (r *response) Body() string {
	return string(r.rawBody)
}

// JSONBody parses the body as a JSON object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *response) JSONBody() (interface{}, error) {
	if r.parsedBody == nil {
		var v interface{}
		err := json.Unmarshal(r.rawBody, &v)
		if err != nil {
			return nil, err
		}
		r.parsedBody = v
	}
	return r.parsedBody, nil
}

// YAMLBody parses the body as a YAML object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *response) YAMLBody() (interface{}, error) {
	if r.parsedBody == nil {
		var v interface{}
		err := yaml.Unmarshal(r.rawBody, &v)
		if err != nil {
			return nil, err
		}
		r.parsedBody = v
	}
	return r.parsedBody, nil
}

func prepareBuilderData(ctx *context.Context) (*builderData, error) {
	bd := &builderData{
		Requests:  make(map[string]*request),
		Responses: make(map[string]*response),
	}

	for k, v := range ctx.Requests() {
		req := v.(*httpprot.Request)
		bd.Requests[k] = &request{
			Request: req.Std(),
			rawBody: req.RawPayload(),
		}
	}

	for k, v := range ctx.Responses() {
		resp := v.(*httpprot.Response)
		bd.Responses[k] = &response{
			Response: resp.Std(),
			rawBody:  resp.RawPayload(),
		}
	}

	return bd, nil
}
