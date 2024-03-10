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

package v

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatFunc(t *testing.T) {
	assert := assert.New(t)

	formatFunc, ok := getFormatFunc("date-time")
	assert.True(ok)
	assert.NotNil(formatFunc)

	formatFunc, ok = getFormatFunc("")
	assert.True(ok)
	assert.NotNil(formatFunc)

	formatFunc, ok = getFormatFunc("url")
	assert.True(ok)
	assert.NotNil(formatFunc)

	formatFunc, ok = getFormatFunc("not-exists")
	assert.False(ok)
	assert.Nil(formatFunc)
}

func TestFormatFuncs(t *testing.T) {
	assert := assert.New(t)

	assert.Nil(standardFormat(""))

	assert.Nil(urlName("abc"))
	assert.NotNil(urlName("abc!"))

	assert.Nil(httpMethod("GET"))
	assert.NotNil(httpMethod("WRONG-METHOD"))

	assert.Nil(httpMethodArray([]string{"GET", "POST"}))
	assert.NotNil(httpMethodArray([]string{"GET", "WRONG-METHOD"}))

	assert.Nil(httpCode(200))
	assert.NotNil(httpCode(999))

	assert.Nil(httpCodeArray([]int{200, 201}))
	assert.NotNil(httpCodeArray([]int{200, 999}))

	assert.Nil(timerfc3339("2019-01-01T00:00:00Z"))
	assert.NotNil(timerfc3339("WRONG-TIME"))

	assert.Nil(duration("1s"))
	assert.NotNil(duration("WRONG-DURATION"))

	assert.Nil(ipcidr("192.0.2.1/24"))
	assert.NotNil(ipcidr("WRONG-IP-CIDR"))

	assert.Nil(ipcidrArray([]string{"192.0.2.1/24"}))
	assert.NotNil(ipcidrArray([]string{"Wrong-IP-CIDR"}))

	assert.Nil(hostport("localhost:8080"))
	assert.NotNil(hostport("WRONG-HOST-PORT"))

	assert.Nil(_regexp("^[a-z]+$"))
	assert.NotNil(_regexp("1223[xfr"))

	assert.Nil(_url("http://localhost:8080"))
	assert.NotNil(_url("http://user^:passwo^rd@foo.com/"))
}
