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

// Package builder implements builder filters.
package builder

import (
	"bytes"
	"text/template"

	sprig "github.com/go-task/slim-sprig"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// Builder is the base HTTP builder.
	Builder struct {
		template *template.Template
	}

	// Spec is the spec of Builder.
	Spec struct {
		LeftDelim  string `json:"leftDelim,omitempty"`
		RightDelim string `json:"rightDelim,omitempty"`
		Template   string `json:"template,omitempty"`
	}
)

// Validate validates the Builder Spec.
func (spec *Spec) Validate() error {
	return nil
}

func (b *Builder) reload(spec *Spec) {
	t := template.New("").Delims(spec.LeftDelim, spec.RightDelim)
	t.Funcs(sprig.TxtFuncMap()).Funcs(extraFuncs)
	b.template = template.Must(t.Parse(spec.Template))
}

func (b *Builder) build(data map[string]interface{}, v interface{}) error {
	var result bytes.Buffer

	if err := b.template.Execute(&result, data); err != nil {
		return err
	}

	return codectool.UnmarshalYAML(result.Bytes(), v)
}

// Status returns status.
func (b *Builder) Status() interface{} {
	return nil
}

// Close closes Builder.
func (b *Builder) Close() {
}

func prepareBuilderData(ctx *context.Context) (map[string]interface{}, error) {
	requests := make(map[string]interface{})
	responses := make(map[string]interface{})

	var defaultReq, defaultResp interface{}
	for k, v := range ctx.Requests() {
		requests[k] = v.ToBuilderRequest(k)
		if k == context.DefaultNamespace {
			defaultReq = requests[k]
		}
	}

	for k, v := range ctx.Responses() {
		responses[k] = v.ToBuilderResponse(k)
		if k == context.DefaultNamespace {
			defaultResp = responses[k]
		}
	}

	return map[string]interface{}{
		"req":       defaultReq,
		"resp":      defaultResp,
		"requests":  requests,
		"responses": responses,
		"data":      ctx.Data(),
		"namespace": ctx.Namespace(),
	}, nil
}
