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

package websocketserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyCopyHeader(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	require.Nil(t, err)
	req.Header.Add("Origin", "origin")
	req.Header.Add("Sec-WebSocket-Protocol", "protocol")
	req.Header.Add("Cookie", "cookie1=1")
	req.RemoteAddr = fmt.Sprintf("%s:%d", req.Host, 8888)

	p := &Proxy{}
	header := p.copyHeader(req)
	assert.Equal("origin", header.Get("Origin"))
	assert.Equal("", header.Get("Sec-WebSocket-Protocol"))
	assert.Equal("cookie1=1", header.Get("Cookie"))
	assert.Equal("127.0.0.1", header.Get(xForwardedFor))
	assert.Equal("127.0.0.1", header.Get(xForwardedHost))
	assert.Equal("http", header.Get(xForwardedProto))
	fmt.Printf("header: %v\n", header)
}

func TestProxyUpgradeRspHeader(t *testing.T) {
	assert := assert.New(t)
	resp := &http.Response{}
	resp.Header = make(http.Header)
	resp.Header.Add("Sec-Websocket-Protocol", "protocol")
	resp.Header.Add("Set-Cookie", "cookie=1")
	resp.Header.Add("Sec-Websocket-Extensions", "extensions")

	p := &Proxy{}
	newHeader := p.upgradeRspHeader(resp)
	assert.Equal("protocol", newHeader.Get("Sec-Websocket-Protocol"))
	assert.Equal("cookie=1", newHeader.Get("Set-Cookie"))
	// only copy protocol and cookie
	assert.Equal("", newHeader.Get("Sec-Websocket-Extensions"))
}

func TestCopyResponse(t *testing.T) {
	testSrv := getTestServer(t, "127.0.0.1:8000")
	defer testSrv.Close()

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8000/start", nil)
	require.Nil(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	copyResp := httptest.NewRecorder()
	copyResponse(copyResp, resp)
	assert.Equal(t, copyResp.Header(), resp.Header)
}
