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

package httpprot

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	err = request.FetchPayload(1024 * 1024)
	assert.Nil(err)

	request.SetPayload([]byte("hello"))
	reader := request.GetPayload()
	data, err := io.ReadAll(reader)
	assert.Nil(err)
	assert.Equal([]byte("hello"), data)
	assert.Equal([]byte("hello"), request.RawPayload())
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

func getRequest(t *testing.T, method string, url string, body io.Reader) *Request {
	stdReq, err := http.NewRequest(method, url, body)
	require.Nil(t, err)
	req, err := NewRequest(stdReq)
	require.Nil(t, err)
	return req

}

func TestRequest2(t *testing.T) {
	assert := assert.New(t)
	{
		// when FetchPayload with -1, payload is stream
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader("123"))

		err := req.FetchPayload(-1)
		assert.Nil(err)
		assert.True(req.IsStream())
		req.Close()
	}

	{
		// test set payload
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", nil)

		req.SetPayload(nil)
		assert.Nil(req.RawPayload())

		req.SetPayload("")
		assert.Equal(http.NoBody, req.GetPayload())

		req.SetPayload("123")
		assert.Equal([]byte("123"), req.RawPayload())
		assert.Equal(int64(3), req.PayloadSize())

		assert.Panics(func() { req.SetPayload(123) })

		req.SetPayload(strings.NewReader("123"))
		assert.True(req.IsStream())

		reader := readers.NewByteCountReader(strings.NewReader("123"))
		req.SetPayload(reader)
		assert.Equal(reader, req.GetPayload())
	}

	{
		// when FetchPayload with -1, payload is stream
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader("123"))

		err := req.FetchPayload(0)
		assert.Nil(err)
		assert.False(req.IsStream())
		req.Close()
	}

	{
		// when ContentLength bigger than FetchPayload
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader("123"))
		req.Std().ContentLength = 3

		err := req.FetchPayload(1)
		assert.Equal(ErrRequestEntityTooLarge, err)
		req.Close()
	}

	{
		// when ContentLength is zero
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", http.NoBody)
		req.Std().ContentLength = 0

		err := req.FetchPayload(100)
		assert.Nil(err)
		req.Close()
	}

	{
		// unknown context length
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader("123123123123"))
		req.Std().ContentLength = -1
		err := req.FetchPayload(100)
		assert.Nil(err)
		req.Close()
	}

	{
		// unknown context length and actual context length longer than FetchPayload
		req := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader("123123123123"))
		req.Std().ContentLength = -1
		err := req.FetchPayload(2)
		assert.Equal(ErrRequestEntityTooLarge, err)
		req.Close()
	}

	{
		stdReq, err := http.NewRequest(http.MethodGet, "/", nil)
		assert.Nil(err)
		assert.Empty(stdReq.URL.Scheme)
		assert.Equal("http", RequestScheme(stdReq))

		stdReq.Header.Add("X-Forwarded-Proto", "fakeProtocol")
		assert.Equal("fakeProtocol", RequestScheme(stdReq))

		stdReq.Header.Del("X-Forwarded-Proto")
		stdReq.TLS = &tls.ConnectionState{}
		assert.Equal("https", RequestScheme(stdReq))
	}
}

func TestBuilderRequest(t *testing.T) {
	assert := assert.New(t)

	{
		request := getRequest(t, http.MethodDelete, "http://127.0.0.1:8888", http.NoBody)
		request.FetchPayload(10000)

		builderReq := request.ToBuilderRequest("DEFAULT").(*builderRequest)
		assert.NotNil(builderReq)
		assert.Equal(http.MethodDelete, builderReq.Method)
		assert.Equal("http://127.0.0.1:8888", builderReq.URL.String())
	}

	{
		request := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader(
			"string",
		))
		request.FetchPayload(10000)

		builderReq := request.ToBuilderRequest("DEFAULT").(*builderRequest)
		assert.NotNil(builderReq)
		assert.Equal([]byte("string"), builderReq.RawBody())
		assert.Equal("string", builderReq.Body())
	}

	{
		request := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader(
			`{"key":"value"}`,
		))
		request.FetchPayload(10000)

		builderReq := request.ToBuilderRequest("DEFAULT").(*builderRequest)
		assert.NotNil(builderReq)
		jsonBody, err := builderReq.JSONBody()
		assert.Nil(err)
		jsonMap := jsonBody.(map[string]interface{})
		assert.Equal("value", jsonMap["key"])
	}

	{
		request := getRequest(t, http.MethodGet, "http://127.0.0.1:8888", strings.NewReader(`
name: test
kind: Test
`,
		))
		request.FetchPayload(10000)

		builderReq := request.ToBuilderRequest("DEFAULT").(*builderRequest)
		assert.NotNil(builderReq)
		yamlBody, err := builderReq.YAMLBody()
		assert.Nil(err)
		yamlMap := yamlBody.(map[string]interface{})
		assert.Equal("test", yamlMap["name"])
		assert.Equal("Test", yamlMap["kind"])
	}
}
