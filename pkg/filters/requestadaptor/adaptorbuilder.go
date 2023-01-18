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

package requestadaptor

import (
	"bytes"
	"fmt"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/filters/builder"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"text/template"

	sprig "github.com/go-task/slim-sprig"
)

const (
	resultBuildErr = "buildErr"
)

type (
	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		Host           string                `json:"host" jsonschema:"omitempty"`
		Method         string                `json:"method" jsonschema:"omitempty,format=httpmethod"`
		Path           *pathadaptor.Spec     `json:"path,omitempty" jsonschema:"omitempty"`
		Header         *httpheader.AdaptSpec `json:"header,omitempty" jsonschema:"omitempty"`
		HeaderTemplate string                `json:"headerTemplate,omitempty" jsonschema:"omitempty"`
		Body           string                `json:"body" jsonschema:"omitempty"`
		Compress       string                `json:"compress" jsonschema:"omitempty"`
		Decompress     string                `json:"decompress" jsonschema:"omitempty"`
		Sign           *SignerSpec           `json:"sign,omitempty" jsonschema:"omitempty"`
		LeftDelim      string                `json:"leftDelim" jsonschema:"omitempty"`
		RightDelim     string                `json:"rightDelim" jsonschema:"omitempty"`
		templates      map[string]*template.Template
		data           map[string]interface{}
	}
)

func (spec *Spec) prepareBuilderData(ctx *context.Context) (err error) {
	spec.data, err = builder.PrepareBuilderData(ctx)
	return err
}

func (spec *Spec) build(key string, v interface{}) error {
	var result bytes.Buffer
	if err := spec.templates[key].Execute(&result, spec.data); err != nil {
		return err
	}
	return codectool.UnmarshalYAML(result.Bytes(), v)
}

// Validate verifies that at least one of the validations is defined.
func (spec *Spec) Validate() error {
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

func (spec *Spec) reload() {
	t := template.New("").Delims(spec.LeftDelim, spec.RightDelim)
	t.Funcs(sprig.TxtFuncMap()).Funcs(builder.ExtraFuncs)
	spec.templates = make(map[string]*template.Template)
	spec.templates["header"] = template.Must(t.Parse(spec.HeaderTemplate))
}

// Status returns status.
func (spec *Spec) Status() interface{} {
	return nil
}

// Close closes Builder.
func (spec *Spec) Close() {
}
