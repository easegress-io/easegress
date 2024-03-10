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
	"fmt"
	"io"
	"net/http"
	"testing"

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

	const yamlConfig = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
`
	proxy := newTestSimpleHttpProxy(yamlConfig, assert)

	stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
	ctx := getCtx(stdr)
	assert.Equal("", proxy.Handle(ctx))
	fmt.Println(ctx.GetOutputResponse().(*httpprot.Response).Status)
	bodyBytes, err := io.ReadAll(ctx.GetOutputResponse().(*httpprot.Response).Body)
	if err != nil {
		fmt.Println(err)
		assert.Fail("read body error")
	}
	fmt.Println(string(bodyBytes))

	// test timeout
	const yamlConfig2 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
timeout: 1ms
`
	proxy = newTestSimpleHttpProxy(yamlConfig2, assert)
	stdr, _ = http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
	ctx = getCtx(stdr)
	assert.NotEmpty(proxy.Handle(ctx), "should timeout")

	// test compression
	const yamlConfig3 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
compression:
  minLength: 1024
`
	proxy = newTestSimpleHttpProxy(yamlConfig3, assert)
	stdr, _ = http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
	ctx = getCtx(stdr)
	assert.Equal("", proxy.Handle(ctx))
	fmt.Println(ctx.GetOutputResponse().(*httpprot.Response).Status)
	_, err = io.ReadAll(ctx.GetOutputResponse().(*httpprot.Response).Body)
	// assert headers contains compression
	header := ctx.GetOutputResponse().(*httpprot.Response).Header()
	encoding := header.Get("Content-Encoding")
	assert.Equal("gzip", encoding, "header should contains Content-Encoding")
	if err != nil {
		fmt.Println(err)
		assert.Fail("read body error")
	}

	// test max body size
	const yamlConfig4 = `
name: simpleHttpProxy
kind: SimpleHTTPProxy
maxBodySize: 1024
`
	proxy = newTestSimpleHttpProxy(yamlConfig4, assert)
	stdr, _ = http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
	ctx = getCtx(stdr)
	assert.Equal("", proxy.Handle(ctx))
	fmt.Println(ctx.GetOutputResponse().(*httpprot.Response).Status)
	_, err = io.ReadAll(ctx.GetOutputResponse().(*httpprot.Response).Body)
	if err != nil {
		fmt.Println(err)
		assert.Fail("read body error")
	}
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
