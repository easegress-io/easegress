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
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestDialServer(t *testing.T) {
	assert := assert.New(t)
	sp := &WebSocketServerPool{
		proxy: &WebSocketProxy{
			spec: &WebSocketProxySpec{},
		},
	}

	svr := &Server{}
	stdr, _ := http.NewRequest(http.MethodGet, "ws://127.0.0.1/ws", nil)
	req, _ := httpprot.NewRequest(stdr)

	svr.URL = "####"
	_, _, err := sp.dialServer(svr, req)
	assert.Error(err)

	svr.URL = "http://127.0.0.1:9999"
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	svr.URL = "https://127.0.0.1:9999"
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	svr.URL = "tcp://127.0.0.1:9999"
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	svr.URL = "ws://127.0.0.1:9999"
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	stdr.Header.Add("Origin", "$#@#@$#$#")
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	stdr.Header.Set("Origin", "http://127.0.0.1/hello")
	stdr.RemoteAddr = "127.0.0.1:8080"
	stdr.TLS = &tls.ConnectionState{}
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)

	stdr.Header.Set("X-Forwarded-For", "192.168.1.1")
	_, _, err = sp.dialServer(svr, req)
	assert.Error(err)
}
