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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

const crsSetupConf = `
SecDefaultAction "phase:1,log,auditlog,pass"
SecDefaultAction "phase:2,log,auditlog,pass"
SecAction \
    "id:900990,\
    phase:1,\
    pass,\
    t:none,\
    nolog,\
    tag:'OWASP_CRS',\
    ver:'OWASP_CRS/4.16.0',\
    setvar:tx.crs_setup_version=4160"
`

func TestWafOwaspRules(t *testing.T) {
	assert := assert.New(t)
	yamlConfig := `
kind: WAF
name: waf-test
ruleGroupName: sqlinjection
`
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, e := filters.NewSpec(nil, "", rawSpec)
	assert.Nil(e, "Failed to create WAF spec")

	p := kind.CreateInstance(spec)
	p.Init()
	defer p.Close()

	assert.Equal(Kind, p.Kind().Name, "Filter kind should be WAF")

	// test no WAF rule group
	ctx := context.New(nil)
	{
		req, err := http.NewRequest("GET", "http://example.com", nil)
		assert.Nil(err, "Failed to create HTTP request")
		setRequest(t, ctx, "default", req)

		result := p.Handle(ctx)
		assert.Equal(result, resultNoController, "Expected result when no WAF controller is found")

		resp := ctx.GetResponse("default").(*httpprot.Response)
		assert.Equal(http.StatusInternalServerError, resp.StatusCode())
	}

	// test with WAF rule group
	{
		controllerConfig := `
kind: WAFController
name: waf-controller
ruleGroups:
  - name: sqlinjection
    rules:
      owaspRules:
        - REQUEST-901-INITIALIZATION.conf
        - REQUEST-942-APPLICATION-ATTACK-SQLI.conf
        - REQUEST-949-BLOCKING-EVALUATION.conf
`
		super := supervisor.NewMock(option.New(), nil, nil, nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err, "Failed to create WAFController spec")
		controller := wafcontroller.WAFController{}
		controller.Init(spec)

		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/test?id=1' OR '1'='1", nil)
		assert.Nil(err, "Failed to create HTTP request")
		setRequest(t, ctx, "controller", req)
		result := p.Handle(ctx)
		assert.Equal(string(protocol.ResultBlocked), result)

		resp := ctx.GetResponse("controller").(*httpprot.Response)
		assert.Equal(http.StatusInternalServerError, resp.StatusCode(), "Expected status code to be 500 Internal Server Error")

		controller.Close()
	}

	{
		controllerConfig := `
kind: WAFController
name: waf-controller
ruleGroups:
  - name: sqlinjection
    rules:
      owaspRules:
        - REQUEST-901-INITIALIZATION.conf
        - REQUEST-942-APPLICATION-ATTACK-SQLI.conf
        - REQUEST-949-BLOCKING-EVALUATION.conf
      customRules: |
        SecDefaultAction "phase:1,log,auditlog,pass"
        SecDefaultAction "phase:2,log,auditlog,pass"
        SecAction \
             "id:900990,\
             phase:1,\
             pass,\
             t:none,\
             nolog,\
             tag:'OWASP_CRS',\
             ver:'OWASP_CRS/4.16.0',\
             setvar:tx.crs_setup_version=4160"
`

		super := supervisor.NewMock(option.New(), nil, nil, nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err, "Failed to create WAFController spec")
		controller := wafcontroller.WAFController{}
		controller.Init(spec)

		req, err := http.NewRequest(http.MethodGet, "http://127.0.1:8080/test?email=alice@example.com", nil)
		assert.Nil(err, "Failed to create HTTP request")
		setRequest(t, ctx, "controller", req)
		result := p.Handle(ctx)
		assert.Equal(string(protocol.ResultOk), result)
	}

	{
		controllerConfig := `
kind: WAFController
name: waf-controller
ruleGroups:
  - name: sqlinjection
    rules:
      ipBlocker:
        whitelist:
          - 136.252.0.2/32
`

		super := supervisor.NewMock(option.New(), nil, nil, nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err, "Failed to create WAFController spec")

		controller := wafcontroller.WAFController{}
		controller.Init(spec)

		req, err := http.NewRequest(http.MethodGet, "http://127.0.1:8080/test", nil)
		assert.Nil(err, "Failed to create HTTP request")
		req.RemoteAddr = "136.252.0.2:12345"
		req.Header.Set("X-Forwarded-For", "136.252.0.2")
		req.Header.Set("X-Real-IP", "136.252.0.2")

		setRequest(t, ctx, "controller", req)
		result := p.Handle(ctx)
		assert.Equal(string(protocol.ResultOk), result, "Expected request to pass through WAF with IPBlocker rules")
	}

	{
		controllerConfig := `
kind: WAFController
name: waf-controller
ruleGroups:
  - name: sqlinjection
    rules:
      ipBlocker:
        blacklist:
          - 158.160.2.1/32
`

		super := supervisor.NewMock(option.New(), nil, nil, nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err, "Failed to create WAFController spec")

		controller := wafcontroller.WAFController{}
		controller.Init(spec)

		req, err := http.NewRequest(http.MethodGet, "http://127.0.1:8080/test", nil)
		assert.Nil(err, "Failed to create HTTP request")
		req.RemoteAddr = "158.160.2.1:12345"
		req.Header.Set("X-Forwarded-For", "158.160.2.1")
		req.Header.Set("X-Real-IP", "158.160.2.1")

		setRequest(t, ctx, "controller", req)
		result := p.Handle(ctx)
		assert.Equal(string(protocol.ResultBlocked), result, "Expected request to pass through WAF with IPBlocker rules")
	}
}
