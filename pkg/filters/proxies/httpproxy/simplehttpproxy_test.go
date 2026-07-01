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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func newTestSimpleHttpProxy(yamlConfig string, assert *assert.Assertions) *SimpleHTTPProxy {
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlConfig), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assert.NoError(err)

	proxy := simpleHTTPProxyKind.CreateInstance(spec).(*SimpleHTTPProxy)
	if proxy == nil {
		assert.Fail("proxy is nil")
	}
	proxy.Init()

	assert.Equal(simpleHTTPProxyKind, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func TestSimpleHttpProxy(t *testing.T) {
	assert := assert.New(t)
	body := "simple http proxy response"
	largeBody := strings.Repeat("large response body ", 128)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/large":
			_, _ = io.WriteString(w, largeBody)
		case "/slow":
			time.Sleep(50 * time.Millisecond)
			_, _ = io.WriteString(w, body)
		default:
			_, _ = io.WriteString(w, body)
		}
	}))
	defer server.Close()

	const yamlConfig = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
`
	proxy := newTestSimpleHttpProxy(yamlConfig, assert)
	defer proxy.Close()

	stdr, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	ctx := getCtx(stdr)
	assert.Equal("", proxy.Handle(ctx))
	resp := ctx.GetOutputResponse().(*httpprot.Response)
	assert.Equal(http.StatusOK, resp.StatusCode())
	assert.Equal(body, string(resp.RawPayload()))

	// test timeout
	const yamlConfig2 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
timeout: 1ms
`
	timeoutProxy := newTestSimpleHttpProxy(yamlConfig2, assert)
	defer timeoutProxy.Close()
	stdr, _ = http.NewRequest(http.MethodGet, server.URL+"/slow", nil)
	ctx = getCtx(stdr)
	assert.Equal(resultServerError, timeoutProxy.Handle(ctx), "should timeout")

	// test compression
	const yamlConfig3 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
compression:
  minLength: 1024
`
	compressionProxy := newTestSimpleHttpProxy(yamlConfig3, assert)
	defer compressionProxy.Close()
	stdr, _ = http.NewRequest(http.MethodGet, server.URL+"/large", nil)
	stdr.Header.Set("Accept-Encoding", "gzip")
	ctx = getCtx(stdr)
	assert.Equal("", compressionProxy.Handle(ctx))
	resp = ctx.GetOutputResponse().(*httpprot.Response)
	header := resp.Header()
	encoding := header.Get("Content-Encoding")
	assert.Equal("gzip", encoding, "header should contains Content-Encoding")
	_, err := io.ReadAll(resp.Body)
	if err != nil {
		assert.Fail("read body error")
	}

	// test max body size
	const yamlConfig4 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
serverMaxBodySize: 1024
`
	maxBodySizeProxy := newTestSimpleHttpProxy(yamlConfig4, assert)
	defer maxBodySizeProxy.Close()
	stdr, _ = http.NewRequest(http.MethodGet, server.URL+"/large", nil)
	ctx = getCtx(stdr)
	assert.Equal(resultServerError, maxBodySizeProxy.Handle(ctx))
	assert.Nil(ctx.GetOutputResponse())
}

func TestSimpleHttpProxyWithRetry(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
retryPolicy: retry
`
	// Create SimpleHTTPProxy with retry policy
	proxy := newTestSimpleHttpProxy(yamlConfig, assert)

	policies := map[string]resilience.Policy{}

	assert.Panics(func() { proxy.InjectResiliencePolicy(policies) })
}
