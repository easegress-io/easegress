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

package httpproxy

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
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
	yamlConfig := `spanName: test`
	spec := &ServerPoolSpec{}
	err := codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// not all server got weight
	yamlConfig = `spanName: test
servers:
- url: http://192.168.1.1
  weight: 10
- url: http://192.168.1.2
  weight: 0
`
	spec = &ServerPoolSpec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// valid spec
	yamlConfig = `spanName: test
failureCodes: [500, 503]
servers:
- url: http://192.168.1.1
  weight: 10
- url: http://192.168.1.2
  weight: 10
`
	spec = &ServerPoolSpec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())
}

func TestInjectResilience(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `spanName: test
retryPolicy: retry
circuitBreakerPolicy: circuitBreaker
servers:
- url: http://192.168.1.1
`
	spec := &ServerPoolSpec{}
	err := codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())

	p := kind.CreateInstance(kind.DefaultSpec()).(*Proxy)
	p.super = supervisor.NewMock(option.New(), nil, nil,
		nil, false, nil, nil)
	sp := NewServerPool(p, spec, "test")
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

	yamlConfig := `spanName: test
memoryCache:
  expiration: 1m
  maxEntryBytes: 100
  codes: [200]
  methods: [GET]
servers:
- url: http://192.168.1.1
`

	spec := &ServerPoolSpec{}
	err := codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.NoError(spec.Validate())

	p := kind.CreateInstance(kind.DefaultSpec()).(*Proxy)
	p.super = supervisor.NewMock(option.New(), nil, nil,
		nil, false, nil, nil)
	sp := NewServerPool(p, spec, "test")
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
	resp.HTTPHeader().Set("X-Foo", "Bar")

	sp.memoryCache.Store(req, resp)
	assert.True(sp.buildResponseFromCache(spCtx))

	req.HTTPHeader().Set("Origin", "http://megaease.com")
	assert.False(sp.buildResponseFromCache(spCtx))

	resp.HTTPHeader().Set("Access-Control-Allow-Origin", "*")
	sp.memoryCache.Store(req, resp)
	assert.True(sp.buildResponseFromCache(spCtx))
}

func TestCopyCORSHeaders(t *testing.T) {
	assert := assert.New(t)

	src, dst := http.Header{}, http.Header{}
	src.Add("Access-Control-Allow-Origin", "http://megaease.com")
	dst.Add("Access-Control-Allow-Origin", "http://megaease.net")

	src.Add("X-Foo", "srcbar")
	dst.Add("X-Foo", "dstbar")

	src.Add("X-Src", "src")
	dst.Add("X-Dst", "dst")

	p := kind.CreateInstance(kind.DefaultSpec()).(*Proxy)
	p.super = supervisor.NewMock(option.New(), nil, nil,
		nil, false, nil, nil)
	sp := NewServerPool(p, &ServerPoolSpec{}, "test")
	dst = sp.mergeResponseHeader(dst, src)

	assert.Equal(1, len(dst.Values("Access-Control-Allow-Origin")))
	assert.Equal("http://megaease.com", dst.Get("Access-Control-Allow-Origin"))

	assert.Equal(2, len(dst.Values("X-Foo")))
	assert.Equal("dstbar", dst.Values("X-Foo")[0])
	assert.Equal("srcbar", dst.Values("X-Foo")[1])

	assert.Equal(1, len(dst.Values("X-Src")))
	assert.Equal("src", dst.Values("X-Src")[0])

	assert.Equal(1, len(dst.Values("X-Dst")))
	assert.Equal("dst", dst.Values("X-Dst")[0])
}

func TestRemoveHopByHopHeader(t *testing.T) {
	assert := assert.New(t)

	h := http.Header{}
	h.Add("Connection", "X-Foo, X-Bar")
	h.Add("X-Foo", "foo")
	h.Add("X-Bar", "bar")
	h.Add("X-Foo-Bar", "foo-bar")

	removeHopByHopHeaders(h)
	assert.Equal(1, len(h))
	assert.Equal("foo-bar", h.Get("X-Foo-Bar"))
}

func TestInFailureCodes(t *testing.T) {
	assert := assert.New(t)

	sp := ServerPool{}
	assert.True(sp.inFailureCodes(500))
	assert.False(sp.inFailureCodes(400))

	sp.failureCodes = map[int]struct{}{400: {}, 404: {}}
	assert.False(sp.inFailureCodes(500))
	assert.True(sp.inFailureCodes(400))
}
