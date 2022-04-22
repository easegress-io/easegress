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
	"net/http"
	"text/template"
	"text/template/parse"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// TemplateContext is template context used by golang lib text/template.
	// Requests and Responses use golang http lib to make sure future update won't influence cutomer codes.
	TemplateContext struct {
		Requests   map[string]*http.Request
		Responses  map[string]*http.Response
		ReqBodies  map[string]*Body
		RespBodies map[string]*Body
	}

	Body struct {
		Body string
		Map  map[string]interface{}
	}

	builder struct {
		template *template.Template
		value    string
	}

	headerBuilder struct {
		key   *builder
		value *builder
	}
)

func getTemplateContext(ctx *context.Context) (*TemplateContext, error) {
	tc := &TemplateContext{
		Requests:   make(map[string]*http.Request),
		Responses:  make(map[string]*http.Response),
		ReqBodies:  make(map[string]*Body),
		RespBodies: make(map[string]*Body),
	}
	for k, v := range ctx.Requests() {
		req := v.(*httpprot.Request).Std()
		tc.Requests[k] = req
	}
	for k, v := range ctx.Responses() {
		resp := v.(*httpprot.Response).Std()
		tc.Responses[k] = resp
	}

	// // process body for request
	// if spec.Body != nil {
	// 	for _, r := range spec.Body.Requests {
	// 		data, err := io.ReadAll(ctx.GetRequest(r.ID).GetPayload())
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		if r.UseMap {
	// 			m := make(map[string]interface{})
	// 			err = json.Unmarshal(data, &m)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			tc.ReqBodies[r.ID] = &Body{string(data), m}
	// 		} else {
	// 			tc.ReqBodies[r.ID] = &Body{string(data), nil}
	// 		}
	// 	}

	// 	// process body for response
	// 	for _, r := range spec.Body.Responses {
	// 		data, err := io.ReadAll(ctx.GetResponse(r.ID).GetPayload())
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		if r.UseMap {
	// 			m := make(map[string]interface{})
	// 			err = json.Unmarshal(data, &m)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			tc.RespBodies[r.ID] = &Body{string(data), m}
	// 		} else {
	// 			tc.RespBodies[r.ID] = &Body{string(data), nil}
	// 		}
	// 	}
	// }
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
