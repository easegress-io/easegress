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

package context

import (
	"crypto/tls"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	protocolHTTP  = "http"
	protocolHTTPS = "https"
	protocolFTP   = "ftp"
)

func TestSingleScheme(t *testing.T) {
	assert := assert.New(t)
	req := &httpRequest{
		std: &http.Request{
			Header: http.Header{},
			URL:    &url.URL{},
		},
	}
	assert.Equal(protocolHTTP, req.Scheme())

	req.std.URL.Scheme = protocolHTTP
	assert.Equal(protocolHTTP, req.Scheme())

	req.std.URL.Scheme = ""
	req.std.TLS = &tls.ConnectionState{}
	assert.Equal(protocolHTTPS, req.Scheme())

	req.std.Header.Add(xForwardedProto, protocolHTTP)
	assert.Equal(protocolHTTP, req.Scheme())

}

func TestMultipleScheme(t *testing.T) {
	assert := assert.New(t)
	req := &httpRequest{
		std: &http.Request{
			Header: http.Header{},
			URL:    &url.URL{},
		},
	}
	req.std.Header.Add(xForwardedProto, protocolHTTP)
	req.std.Header.Add(xForwardedProto, protocolHTTPS)
	req.std.Header.Add(xForwardedProto, protocolFTP)
	assert.Equal(protocolHTTP, req.Scheme())
	var v []string
	v = append(v, protocolHTTP)
	v = append(v, protocolHTTPS)
	v = append(v, protocolFTP)
	req.std.Header.Set(xForwardedProto, strings.Join(v, ","))
	assert.Equal(protocolFTP, req.Scheme())
}
