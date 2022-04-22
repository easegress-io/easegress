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
	"fmt"
	"io"
	"net/http"
	"text/template"
	"text/template/parse"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"gopkg.in/yaml.v3"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// TemplateContext is template context used by golang lib text/template.
	// Requests and Responses use golang http lib to make sure future update won't influence cutomer codes.
	TemplateContext struct {
		Requests       map[string]*http.Request
		Responses      map[string]*http.Response
		RequestBodies  map[string]*Body
		ResponseBodies map[string]*Body
	}

	// Body is a struct used to store request and response body.
	// Body method will panic if meet read error or json marshal error or yaml marshal error.
	Body struct {
		p    fetchGetPayloader
		data []byte
		m    map[string]interface{}
	}

	builder struct {
		template *template.Template
		value    string
	}

	headerBuilder struct {
		key   *builder
		value *builder
	}

	fetchGetPayloader interface {
		FetchPayload() (int, error)
		GetPayload() io.Reader
	}
)

func newBody(p fetchGetPayloader) *Body {
	return &Body{p: p}
}

// Byte return body bytes.
func (body *Body) Byte() []byte {
	if body.data == nil {
		_, err := body.p.FetchPayload()
		if err != nil {
			panic(fmt.Errorf("fetch payload error: %s", err))
		}
		data, err := io.ReadAll(body.p.GetPayload())
		if err != nil {
			panic(fmt.Errorf("read body error: %s", err))
		}
		body.data = data
	}
	return body.data
}

// Byte return body bytes as string.
func (body *Body) String() string {
	if body.data == nil {
		body.Byte()
	}
	return string(body.data)
}

// Byte return body bytes as json map.
func (body *Body) JsonMap() map[string]interface{} {
	if body.m == nil {
		body.m = make(map[string]interface{})
		err := json.Unmarshal(body.Byte(), &body.m)
		if err != nil {
			panic(fmt.Errorf("json unmarshal error: %s", err))
		}
	}
	return body.m
}

// Byte return body bytes as yaml map.
func (body *Body) YamlMap() map[string]interface{} {
	if body.m == nil {
		body.m = make(map[string]interface{})
		err := yaml.Unmarshal(body.Byte(), &body.m)
		if err != nil {
			panic(fmt.Errorf("yaml unmarshal error: %s", err))
		}
	}
	return body.m
}

func getTemplateContext(ctx *context.Context) (*TemplateContext, error) {
	tc := &TemplateContext{
		Requests:       make(map[string]*http.Request),
		Responses:      make(map[string]*http.Response),
		RequestBodies:  make(map[string]*Body),
		ResponseBodies: make(map[string]*Body),
	}
	for k, v := range ctx.Requests() {
		httpreq := v.(*httpprot.Request)
		req := httpreq.Std()
		tc.Requests[k] = req
		tc.RequestBodies[k] = newBody(httpreq)
	}
	for k, v := range ctx.Responses() {
		httpresp := v.(*httpprot.Response)
		resp := httpresp.Std()
		tc.Responses[k] = resp
		tc.ResponseBodies[k] = newBody(httpresp)
	}

	return tc, nil
}

func getBuilder(templateStr string) *builder {
	temp := template.Must(template.New(templateStr).Parse(templateStr))
	for _, node := range temp.Root.Nodes {
		if node.Type() == parse.NodeAction {
			return &builder{temp, ""}
		}
	}
	return &builder{nil, templateStr}
}

func (b *builder) build(ctx interface{}) (string, error) {
	if b.template == nil {
		return b.value, nil
	}

	var result bytes.Buffer
	err := b.template.Execute(&result, ctx)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}
