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
	"net/http/httptest"
	"testing"

	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestWebSocketSpecValidate(t *testing.T) {
	assert := assert.New(t)

	// no main pool
	yamlConfig := `
name: proxy
kind: Proxy
pools:
- filter:
    headers:
      "X-Test":
        exact: testheader
  servers:
  - url: ws://127.0.0.1:9095
`
	spec := &WebSocketProxySpec{}
	err := codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// no servers and service discovery
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
`
	spec = &WebSocketProxySpec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// two main pools
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
  - url: ws://127.0.0.1:9095
- servers:
  - url: ws://127.0.0.2:9096
`
	spec = &WebSocketProxySpec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())
}

func newTestWebSocketProxy(yamlConfig string, assert *assert.Assertions) *WebSocketProxy {
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlConfig), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assert.NoError(err)

	proxy := kindWebSocketProxy.CreateInstance(spec).(*WebSocketProxy)
	proxy.Init()

	assert.Equal(kindWebSocketProxy, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func TestWebSocketProxy(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
name: wsproxy
kind: WebSocketProxy
pools:
- servers:
  - url: ws://127.0.0.1:9095
  - url: ws://127.0.0.1:9096
  - url: ws://127.0.0.1:9097
  loadBalance:
    policy: roundRobin
- filter:
    headers:
      "X-Test":
        exact: testheader
  servers:
  - url: ws://127.0.0.2:9095
  - url: ws://127.0.0.2:9096
  - url: ws://127.0.0.2:9097
  - url: ws://127.0.0.2:9098
  loadBalance:
    policy: roundRobin
  timeout: 10ms
- filter:
    headers:
      "X-Test":
        exact: stream
  servers:
  - url: ws://127.0.0.2:9095
  - url: ws://127.0.0.2:9096
  - url: ws://127.0.0.2:9097
  - url: ws://127.0.0.2:9098
  loadBalance:
    policy: roundRobin
`
	proxy := newTestWebSocketProxy(yamlConfig, assert)

	assert.Equal(2, len(proxy.candidatePools))

	assert.NotNil(proxy.Status())

	{
		stdr, _ := http.NewRequest(http.MethodGet, "wss://www.megaease.com", nil)
		ctx := getCtx(stdr)
		assert.Equal(resultInternalError, proxy.Handle(ctx))
	}

	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Test", "testheader")
		ctx := getCtx(stdr)
		assert.Equal(resultInternalError, proxy.Handle(ctx))
	}

	proxy.Close()

	proxy2 := &WebSocketProxy{}
	proxy2.spec = proxy.spec
	proxy2.Inherit(proxy)
	proxy = proxy2

	{
		stdr, _ := http.NewRequest(http.MethodGet, "wss://www.megaease.com", nil)
		ctx := getCtx(stdr)
		ctx.SetData("HTTP_RESPONSE_WRITER", httptest.NewRecorder())
		assert.Equal(resultClientError, proxy.Handle(ctx))
	}
}
