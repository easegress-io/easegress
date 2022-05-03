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
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"gopkg.in/yaml.v3"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// HTTPBuilder is the base HTTP builder.
	HTTPBuilder struct {
		template *template.Template
	}

	// Spec is the spec of HTTPBuilder.
	Spec struct {
		LeftDelim  string `yaml:"leftDelim" jsonschema:"omitempty"`
		RightDelim string `yaml:"rightDelim" jsonschema:"omitempty"`
		Template   string `yaml:"template" jsonschema:"required"`
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

var funcs = template.FuncMap{
	"split": func(sep, str string) []string {
		return strings.Split(str, sep)
	},
}

func (b *HTTPBuilder) reload(spec *Spec) {
	t := template.New("").Delims(spec.LeftDelim, spec.RightDelim)
	t.Funcs(funcs)
	b.template = template.Must(t.Parse(spec.Template))
}

func (b *HTTPBuilder) build(data *builderData, v interface{}) error {
	var result bytes.Buffer

	if err := b.template.Execute(&result, data); err != nil {
		return err
	}

	if err := yaml.NewDecoder(&result).Decode(v); err != nil {
		return err
	}

	return nil
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
