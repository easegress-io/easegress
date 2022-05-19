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
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest(t *testing.T) {
	assert := assert.New(t)

	// nil request
	request, err := NewRequest(nil)
	assert.Nil(err)
	assert.NotNil(request.Std())
	request.Close()

	// not nil request
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:80", strings.NewReader("body string"))
	assert.Nil(err)
	request, err = NewRequest(req)
	assert.Nil(err)

	// payload related
	l, err := request.FetchPayload()
	assert.Nil(err)
	assert.Equal(len("body string"), l)

	request.SetPayload([]byte("hello"))
	reader := request.GetPayload()
	data, err := io.ReadAll(reader)
	assert.Nil(err)
	assert.Equal([]byte("hello"), data)
	assert.Equal([]byte("hello"), request.RawPayload())
	assert.Equal(5, request.PayloadLength())
	assert.NotZero(request.MetaSize())

	// header
	assert.IsType(http.Header{}, request.HTTPHeader())
	request.HTTPHeader().Set("foo", "bar")
	assert.Equal("bar", request.Header().Get("foo"))

	assert.Equal("http", RequestScheme(request.Std()))
	assert.Equal("http", request.Scheme())
	assert.Empty(request.RealIP())
	assert.Equal(req, request.Std())
	assert.IsType(&url.URL{}, request.URL())
	assert.Equal("HTTP/1.1", request.Proto())
	assert.Equal(http.MethodGet, request.Method())

	// cookie
	cookie := &http.Cookie{
		Name:   "key",
		Value:  "value",
		MaxAge: 300,
	}
	request.AddCookie(cookie)
	assert.Equal(cookie.Name, request.Cookies()[0].Name)

	c, err := request.Cookie("key")
	assert.Nil(err)
	assert.Equal(cookie.Name, c.Name)
	assert.Equal(cookie.Value, c.Value)

	assert.NotNil(request.Context())
	cancel()
	assert.Equal(context.Canceled, request.Context().Err())

	request.SetMethod(http.MethodPost)
	assert.Equal(http.MethodPost, request.Method())

	assert.Equal("127.0.0.1:80", request.Host())
	request.SetHost("localhost:8080")
	assert.Equal("localhost:8080", request.Host())

	request.SetPath("/foo/bar")
	assert.Equal("/foo/bar", request.Path())
}
