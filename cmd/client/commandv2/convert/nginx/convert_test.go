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

package nginx

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/specs"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/builder"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestAdaptor(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("GET", "http://example.com", strings.NewReader("test"))
	req.Header.Set("Content-Length", "4")
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth("user", "pass")
	req.RemoteAddr = "localhost:8080"
	req.RequestURI = "/apis/v1"
	assert.Nil(err)
	ctx := newContext(t, req)
	info := &ProxyInfo{
		SetHeaders: map[string]string{
			"X-Host":         "$host",
			"X-Hostname":     "$hostname",
			"X-Content":      "$content_length",
			"X-Content-Type": "$content_type",
			"X-Remote-Addr":  "$remote_addr",
			"X-Remote-User":  "$remote_user",
			"X-Request-Body": "$request_body",
			"X-Method":       "$request_method",
			"X-Request-URI":  "$request_uri",
			"X-Scheme":       "$scheme",
		},
	}
	spec := getRequestAdaptor(info)
	ra := filters.GetKind(builder.RequestAdaptorKind).CreateInstance(spec)
	ra.Init()
	ra.Handle(ctx)
	h := ctx.GetInputRequest().(*httpprot.Request).Header()
	expected := map[string]string{
		"X-Host":         "example.com",
		"X-Hostname":     "example.com",
		"X-Content":      "4",
		"X-Content-Type": "text/plain",
		"X-Remote-Addr":  "localhost:8080",
		"X-Remote-User":  "user",
		"X-Request-Body": "test",
		"X-Method":       "GET",
		"X-Request-URI":  "/apis/v1",
		"X-Scheme":       "http",
	}
	for k, v := range expected {
		assert.Equal(v, h.Get(k), fmt.Sprintf("header %s", k))
	}
}

func TestConvertConfig(t *testing.T) {
	options := &Options{
		ResourcePrefix: "test-convert",
	}
	options.init()
	conf := `
servers:
- port: 8080
  address: localhost
  https: true
  caCert: caCertBase64Str
  certs:
    cert1: cert1Base64Str
  keys:
    cert1: key1Base64Str
  rules:
  - hosts:
    - value: www.example.com
      isRegexp: false
    - isRegexp: true
      value: '.*\.example\.com'
    paths:
    - path: /apis
      type: prefix
      backend:
        servers:
        - server: http://localhost:8880
          weight: 1
        - server: http://localhost:8881
          weight: 2
        setHeaders:
          X-Path: apis
        gzipMinLength: 1000
    - path: /exact
      type: exact
      backend:
        servers:
        - server: http://localhost:9999
          weight: 1
    - path: /regexp
      type: regexp
      backend:
        servers:
        - server: http://localhost:7777
          weight: 1
    - path: /case-insensitive-regexp
      type: caseInsensitiveRegexp
      backend:
        servers:
        - server: http://localhost:6666
          weight: 1
    - path: /websocket
      type: prefix
      backend:
        servers:
        - server: https://localhost:9090
          weight: 1
        setHeaders:
          Connection: $connection_upgrade
          Upgrade: $http_upgrade
`
	config := &Config{}
	err := codectool.Unmarshal([]byte(conf), config)
	assert.Nil(t, err)
	httpServers, pipelines, err := convertConfig(options, config)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(httpServers))
	assert.Equal(t, 5, len(pipelines))
	serverYaml := `
name: test-convert-8080
kind: HTTPServer
https: true
caCertBase64: caCertBase64Str
certs:
  cert1: cert1Base64Str
keys:
  cert1: key1Base64Str
port: 8080
address: localhost
rules:
- hosts:
  - value: www.example.com
    isRegexp: false
  - isRegexp: true
    value: '.*\.example\.com'
  paths:
  - path: /exact
    backend: test-convert-exact
  - pathPrefix: /websocket
    backend: test-convert-websocket
    clientMaxBodySize: -1
  - pathPrefix: /apis
    backend: test-convert-apis
  - pathRegexp: /regexp
    backend: test-convert-regexp
  - pathRegexp: (?i)/case-insensitive-regexp
    backend: test-convert-caseinsensitiveregexp
`
	expected := specs.NewHTTPServerSpec("test-convert-8080")
	err = codectool.UnmarshalYAML([]byte(serverYaml), expected)
	assert.Nil(t, err)
	assert.Equal(t, expected, httpServers[0])

	pipelineApis := `
name: test-convert-apis
kind: Pipeline
filters:
  - kind: RequestAdaptor
    name: request-adaptor
    template: |
      header:
          set:
              X-Path: apis
  - compression:
      minLength: 1000
    kind: Proxy
    name: proxy
    pools:
      - loadBalance:
          policy: weightedRandom
        servers:
          - url: http://localhost:8880
            weight: 1
          - url: http://localhost:8881
            weight: 2
`
	pipelineExact := `
name: test-convert-exact
kind: Pipeline
filters:
  - kind: Proxy
    name: proxy
    pools:
      - loadBalance:
          policy: roundRobin
        servers:
          - url: http://localhost:9999
            weight: 1
`
	pipelineRegexp := `
name: test-convert-regexp
kind: Pipeline
filters:
  - kind: Proxy
    name: proxy
    pools:
      - loadBalance:
          policy: roundRobin
        servers:
          - url: http://localhost:7777
            weight: 1
`
	pipelineCIReg := `
name: test-convert-caseinsensitiveregexp
kind: Pipeline
filters:
  - kind: Proxy
    name: proxy
    pools:
      - loadBalance:
          policy: roundRobin
        servers:
          - url: http://localhost:6666
            weight: 1
`
	pipelineWebsocket := `
name: test-convert-websocket
kind: Pipeline
filters:
    - kind: WebSocketProxy
      name: websocket
      pools:
        - loadBalance:
            policy: roundRobin
          servers:
            - url: wss://localhost:9090
              weight: 1
`
	for i, yamlStr := range []string{pipelineApis, pipelineExact, pipelineRegexp, pipelineCIReg, pipelineWebsocket} {
		spec := specs.NewPipelineSpec("")
		err = codectool.UnmarshalYAML([]byte(yamlStr), spec)
		assert.Nil(t, err, i)
		for j, f := range spec.Filters {
			compareFilter(t, f, pipelines[i].Filters[j], fmt.Sprintf("%d filter in %d pipeline", j, i))
		}
	}
}

func compareFilter(t *testing.T, f1 map[string]interface{}, f2 map[string]interface{}, msg string) {
	d1, err := codectool.MarshalYAML(f1)
	assert.Nil(t, err, msg)
	d2, err := codectool.MarshalYAML(f2)
	assert.Nil(t, err, msg)

	var specFn func() interface{}
	switch f1["kind"] {
	case "Proxy":
		specFn = func() interface{} {
			return specs.NewProxyFilterSpec("")
		}
	case "RequestAdaptor":
		specFn = func() interface{} {
			return specs.NewRequestAdaptorFilterSpec("")
		}
	case "WebSocketProxy":
		specFn = func() interface{} {
			return specs.NewWebsocketFilterSpec("")
		}
	default:
		t.Errorf("filter kind %s is not compared", f1["kind"])
		return
	}
	s1 := specFn()
	err = codectool.UnmarshalYAML(d1, s1)
	assert.Nil(t, err, msg)
	s2 := specFn()
	err = codectool.Unmarshal(d2, s2)
	assert.Nil(t, err, msg)
	assert.Equal(t, s1, s2)
}
