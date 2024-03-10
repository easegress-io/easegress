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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAddressPort(t *testing.T) {
	testCases := []struct {
		listen  string
		address string
		port    int
		err     bool
	}{
		{listen: "127.0.0.1:8000", address: "127.0.0.1", port: 8000, err: false},
		{listen: "127.0.0.1", address: "127.0.0.1", port: 80, err: false},
		{listen: "8000", address: "", port: 8000, err: false},
		{listen: "*:8000", address: "*", port: 8000, err: false},
		{listen: "localhost:8000", address: "localhost", port: 8000, err: false},
		{listen: "[::]:8000", address: "[::]", port: 8000, err: false},
		{listen: "[::1]", address: "[::1]", port: 80, err: false},
		{listen: "", address: "", port: 0, err: true},
		{listen: "[::", address: "", port: 0, err: true},
		{listen: "[::]:", address: "", port: 0, err: true},
		{listen: "[::]:8888", address: "[::]", port: 8888, err: false},
	}
	for _, tc := range testCases {
		msg := fmt.Sprintf("%v", tc)
		address, port, err := splitAddressPort(tc.listen)
		assert.Equal(t, tc.address, address, msg)
		assert.Equal(t, tc.port, port, msg)
		if tc.err {
			assert.NotNil(t, err, msg)
		} else {
			assert.Nil(t, err, msg)
		}
	}
}

func newDirective(d string, args ...string) *Directive {
	return &Directive{
		Directive: d,
		Args:      args,
	}
}

func TestEnvProcessErrors(t *testing.T) {
	// gzip
	{
		testCases := []struct {
			gzip *Directive
			len  *Directive
			res  int
		}{
			{gzip: newDirective("gzip", "invalid"), len: nil, res: 0},
			{gzip: newDirective("gzip", "on"), len: nil, res: 20},
			{gzip: newDirective("gzip", "on"), len: newDirective("gzip_min_length", "200"), res: 200},
			{gzip: newDirective("gzip", "on"), len: newDirective("gzip_min_length", "invalid"), res: 20},
			{gzip: newDirective("gzip", "on"), len: newDirective("gzip_min_length", "-1"), res: 20},
		}
		for i, tc := range testCases {
			gzip := &GzipEnv{
				Gzip:          tc.gzip,
				GzipMinLength: tc.len,
			}
			got := processGzip(gzip)
			assert.Equal(t, tc.res, got, "case", i)
		}
	}

	// ssl
	{
		certs := []*Directive{newDirective("ssl_certificate", "cert1"), newDirective("ssl_certificate", "cert2")}
		keys := []*Directive{newDirective("ssl_certificate_key", "key1")}
		_, _, err := processSSLCertificates(certs, keys)
		assert.NotNil(t, err)

		certs = []*Directive{newDirective("ssl_certificate", "cert1")}
		keys = []*Directive{newDirective("ssl_certificate_key", "key1"), newDirective("ssl_certificate_key", "key2")}
		_, _, err = processSSLCertificates(certs, keys)
		assert.NotNil(t, err)

		certs = []*Directive{newDirective("ssl_certificate", "cert1")}
		keys = []*Directive{newDirective("ssl_certificate_key", "key1")}
		_, _, err = processSSLCertificates(certs, keys)
		assert.NotNil(t, err)
	}

	// server name
	{
		testCases := []struct {
			server     *Directive
			hostValues []string
			isRegexp   []bool
			err        bool
		}{
			{server: newDirective("server_name", "~www.example.com$"), hostValues: []string{"www.example.com$"}, isRegexp: []bool{true}, err: false},
			{server: newDirective("server_name", "~["), hostValues: []string{}, isRegexp: []bool{}, err: true},
			{server: newDirective("server_name", "*.example.*"), hostValues: []string{}, isRegexp: []bool{}, err: true},
		}
		for i, tc := range testCases {
			serverNames, err := processServerName(tc.server)
			assert.Equal(t, len(tc.hostValues), len(serverNames), "case", i)
			for i := 0; i < len(tc.hostValues); i++ {
				assert.Equal(t, tc.hostValues[i], serverNames[i].Value, "case", i)
				assert.Equal(t, tc.isRegexp[i], serverNames[i].IsRegexp, "case", i)
			}
			if tc.err {
				assert.NotNil(t, err, "case", i)
			} else {
				assert.Nil(t, err, "case", i)
			}
		}
	}
}
