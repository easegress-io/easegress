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

package waf

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Kind is the kind of WAF filter.
	Kind = "WAF"
	// resultNoController is the result when no WAF controller is found.
	resultNoController = "noWAFControllerError"
)

type (
	// WAF is the filter that implements Web Application Firewall (WAF) functionality.
	WAF struct {
		spec *Spec
	}

	// Spec is the specification for the WAF filter.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		// RuleGroupName is the name of the rule group to use for WAF.
		RuleGroupName string `json:"ruleGroupName" jsonschema:"required"`
	}
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Web Application Firewall (WAF) filter to protect web applications from attacks.",
	Results:     []string{string(wafcontroller.ResultRuleGroupNotFoundError), resultNoController},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &WAF{
			spec: spec.(*Spec),
		}
	},
}

func init() {
	filters.Register(kind)
}

func (w *WAF) Name() string {
	return w.spec.Name()
}

func (w *WAF) Kind() *filters.Kind {
	return kind
}

func (w *WAF) Spec() filters.Spec {
	return w.spec
}

func (w *WAF) Init() {
	w.reload()
}

func (w *WAF) Inherit(previousGeneration filters.Filter) {
	w.Init()
}

func (w *WAF) reload() {}

func (w *WAF) Handle(context *context.Context) (result string) {
	handler, err := wafcontroller.GetGlobalWAFController()
	if err != nil {
		setErrResponse(context, err)
		return resultNoController
	}
	return handler.Handle(context, w.spec.RuleGroupName)
}

func setErrResponse(ctx *context.Context, err error) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	resp.SetStatusCode(http.StatusInternalServerError)
	errMsg := map[string]string{
		"Message": err.Error(),
	}
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
	ctx.SetOutputResponse(resp)
}

func (w *WAF) Status() interface{} {
	return nil
}

func (w *WAF) Close() {}
