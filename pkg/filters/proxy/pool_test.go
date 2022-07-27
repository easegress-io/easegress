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

package proxy

import (
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestServerPoolError(t *testing.T) {
	assert := assert.New(t)

	spe := serverPoolError{http.StatusServiceUnavailable, resultInternalError}
	assert.Equal(http.StatusServiceUnavailable, spe.Code())
	assert.Equal("server pool error, status code=503, result="+resultInternalError, spe.Error())
}

func TestServerPoolSpecValidate(t *testing.T) {
	assert := assert.New(t)

	// no servers and not service discovery
	yamlSpec := `spanName: test`
	spec := &ServerPoolSpec{}
	err := yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// not all server got weight
	yamlSpec = `spanName: test
servers:
- url: http://192.168.1.1
  weight: 10
- url: http://192.168.1.2
  weight: 0
`
	spec = &ServerPoolSpec{}
	err = yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// valid spec
	yamlSpec = `spanName: test
failureCodes: [500, 503]
servers:
- url: http://192.168.1.1
  weight: 10
- url: http://192.168.1.2
  weight: 10
`
	spec = &ServerPoolSpec{}
	err = yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())
}

func TestInjectResilience(t *testing.T) {
	assert := assert.New(t)

	yamlSpec := `spanName: test
retryPolicy: retry
circuitBreakerPolicy: circuitBreaker
servers:
- url: http://192.168.1.1
`
	spec := &ServerPoolSpec{}
	err := yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())

	sp := NewServerPool(nil, spec, "test")
	policies := map[string]resilience.Policy{}

	assert.Panics(func() { sp.InjectResiliencePolicy(policies) })

	policies["retry"] = &resilience.CircuitBreakerPolicy{}
	assert.Panics(func() { sp.InjectResiliencePolicy(policies) })

	policies["retry"] = &resilience.RetryPolicy{}
	assert.Panics(func() { sp.InjectResiliencePolicy(policies) })

	policies["circuitBreaker"] = &resilience.RetryPolicy{}
	assert.Panics(func() { sp.InjectResiliencePolicy(policies) })

	policies["circuitBreaker"] = &resilience.CircuitBreakerPolicy{}
	assert.NotPanics(func() { sp.InjectResiliencePolicy(policies) })

	assert.NotNil(sp.retryWrapper)
	assert.NotNil(sp.circuitBreakerWrapper)
}

func TestBuildResponseFromCache(t *testing.T) {
	assert := assert.New(t)

	yamlSpec := `spanName: test
memoryCache:
  expiration: 1m
  maxEntryBytes: 100
  codes: [200]
  methods: [GET]
servers:
- url: http://192.168.1.1
`

	spec := &ServerPoolSpec{}
	err := yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())

	sp := NewServerPool(nil, spec, "test")
	spCtx := &serverPoolContext{
		Context: context.New(tracing.NoopSpan),
	}
	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)
	spCtx.req = req

	spCtx.SetRequest(context.DefaultNamespace, req)

	assert.False(sp.buildResponseFromCache(spCtx))

	resp, _ := httpprot.NewResponse(nil)
	resp.SetPayload([]byte("0123456789A"))

	sp.memoryCache.Store(req, resp)
	assert.True(sp.buildResponseFromCache(spCtx))

	req.HTTPHeader().Set("Origin", "http://megaease.com")
	assert.True(sp.buildResponseFromCache(spCtx))
}

func TestCopyCORSHeaders(t *testing.T) {
	assert := assert.New(t)

	var (
		src = http.Header{}
		dst = http.Header{}
	)

	result := copyCORSHeaders(dst, src)
	assert.False(result)

	src.Set("Access-Control-Allow-Origin", "http://megaease.com")
	result = copyCORSHeaders(dst, src)
	assert.True(result)
	assert.Equal("http://megaease.com", dst.Get("Access-Control-Allow-Origin"))
	assert.Equal(1, strings.Count(dst.Get("Vary"), "Origin"))

	src.Set("Access-Control-Expose-Headers", "X-Foo")
	src.Set("Access-Control-Allow-Credentials", "true")
	result = copyCORSHeaders(dst, src)
	assert.True(result)
	assert.Equal("X-Foo", dst.Get("Access-Control-Expose-Headers"))
	assert.Equal("true", dst.Get("Access-Control-Allow-Credentials"))

	assert.Equal(1, strings.Count(dst.Get("Vary"), "Origin"))
}

func TestUseService(t *testing.T) {
	assert := assert.New(t)

	yamlSpec := `spanName: test
serverTags: [a1, a2]
servers:
- url: http://192.168.1.1
`

	spec := &ServerPoolSpec{}
	err := yaml.Unmarshal([]byte(yamlSpec), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())

	sp := NewServerPool(nil, spec, "test")
	svr := sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.1", svr.URL)

	sp.useService(nil)
	assert.Equal("http://192.168.1.1", svr.URL)

	sp.useService(map[string]*serviceregistry.ServiceInstanceSpec{
		"2": {
			Address: "192.168.1.2",
			Tags:    []string{"a2"},
			Port:    80,
		},
		"3": {
			Address: "192.168.1.3",
			Tags:    []string{"a3"},
			Port:    80,
		},
	})
	svr = sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.2:80", svr.URL)
	svr = sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.2:80", svr.URL)
}
