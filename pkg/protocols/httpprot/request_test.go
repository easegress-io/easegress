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

package httpprot

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:80", strings.NewReader("body string"))
	assert.Nil(err)

	request := NewRequest(req)
	assert.Equal(req, request.Std())
	assert.Equal("", request.RealIP())
	assert.Equal("HTTP/1.1", request.Proto())
	assert.Equal(req.URL, request.URL())
	assert.Equal(http.MethodGet, request.Method())

	// when SetMethod, both Request and http.Request method should be changed
	request.SetMethod(http.MethodDelete)
	assert.Equal(http.MethodDelete, request.Method())
	assert.Equal(http.MethodDelete, req.Method)

	// Request and http.Request should share same cookie
	assert.Equal([]*http.Cookie{}, request.Cookies())
	assert.Equal([]*http.Cookie{}, req.Cookies())

	request.AddCookie(&http.Cookie{
		Name:  "cookie",
		Value: "123",
	})
	cookie, err := request.Cookie("cookie")
	assert.Nil(err)
	assert.Equal("123", cookie.Value)
	cookie, err = req.Cookie("cookie")
	assert.Nil(err)
	assert.Equal("123", cookie.Value)

	// Request and http.Request should share same header
	assert.Equal(req.Header.Get("Cookie"), request.Header().Get("Cookie"))

	// Payload reader and http.Request reader should both work
	reader := request.GetPayloadReader()
	data, err := io.ReadAll(reader)
	assert.Nil(err)
	assert.Equal("body string", string(data))

	data, err = io.ReadAll(req.Body)
	assert.Nil(err)
	assert.Equal("body string", string(data))

	// check context
	ctx, cancel := context.WithCancel(request.Context())
	request.WithContext(ctx)
	assert.Nil(request.Context().Err())

	cancel()
	assert.Equal(context.Canceled, request.Context().Err())
	assert.Equal(context.Canceled, request.Std().Context().Err())
}
