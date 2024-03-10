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

package proxies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	assert := assert.New(t)

	svr := Server{
		URL:    "abc",
		Tags:   []string{"test", "standby"},
		Weight: 10,
	}

	assert.Equal("abc,[test standby],10", svr.String())
}

func TestCheckAddrPattern(t *testing.T) {
	assert := assert.New(t)

	server := Server{}

	// regard invalid url as IP:port
	server.URL = "@@+=%^httpsidfssjflsdkjfsjf"
	server.CheckAddrPattern()
	assert.False(server.AddrIsHostName, "address should be IP:port")

	server.URL = "http://127.0.0.1:1111"
	server.CheckAddrPattern()
	assert.False(server.AddrIsHostName, "address should be IP:port")

	server.URL = "https://127.0.0.1:1111"
	server.CheckAddrPattern()
	assert.False(server.AddrIsHostName, "address should be IP:port")

	server.URL = "https://[FE80:CD00:0000:0CDE:1257:0000:211E:729C]:1111"
	server.CheckAddrPattern()
	assert.False(server.AddrIsHostName, "address should be IP:port")

	server.URL = "https://www.megaease.com:1111"
	server.CheckAddrPattern()
	assert.True(server.AddrIsHostName, "address should be host name")

	server.URL = "https://www.megaease.com"
	server.CheckAddrPattern()
	assert.True(server.AddrIsHostName, "address should be host name")

	server.URL = "faas-func-name.default.example.com"
	server.CheckAddrPattern()
	assert.True(server.AddrIsHostName, "address should not be IP:port")
}
