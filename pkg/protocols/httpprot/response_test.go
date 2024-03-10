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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/util/readers"
	"github.com/stretchr/testify/assert"
)

func TestNilResponse(t *testing.T) {
	assert := assert.New(t)

	resp, err := NewResponse(nil)
	assert.Nil(err)
	defer resp.Close()
	resp.HTTPHeader().Set("foo", "bar")
	assert.Equal("bar", resp.Header().Get("foo"))

	err = resp.FetchPayload(1024 * 1024)
	assert.Nil(err)

	resp.SetPayload([]byte("hello"))
	r := resp.GetPayload()
	data, err := io.ReadAll(r)
	assert.Nil(err)
	assert.Equal([]byte("hello"), data)
	assert.Equal([]byte("hello"), resp.RawPayload())

	assert.NotNil(resp.Std())
	assert.NotZero(resp.MetaSize())
	assert.Equal(http.StatusOK, resp.StatusCode())

	resp.SetStatusCode(http.StatusBadRequest)
	assert.Equal(http.StatusBadRequest, resp.StatusCode())

	cookie := &http.Cookie{
		Name:   "key",
		Value:  "value",
		MaxAge: 300,
	}
	resp.SetCookie(cookie)
	assert.Equal(cookie.Name, resp.Cookies()[0].Name)
}

func TestResponse(t *testing.T) {
	assert := assert.New(t)

	rw := httptest.NewRecorder()
	l, err := rw.Write([]byte("hello"))
	assert.Nil(err)
	assert.Equal(5, l)

	r := rw.Result()
	resp, err := NewResponse(r)
	assert.Nil(err)
	defer resp.Close()
	resp.HTTPHeader().Set("foo", "bar")

	err = resp.FetchPayload(1024 * 1024)
	assert.Nil(err)

	resp.SetPayload([]byte("new"))
	reader := resp.GetPayload()
	data, err := io.ReadAll(reader)
	assert.Nil(err)
	assert.Equal([]byte("new"), data)
	assert.Equal([]byte("new"), resp.RawPayload())

	assert.NotNil(resp.Std())
	assert.NotZero(resp.MetaSize())
	assert.Equal(http.StatusOK, resp.StatusCode())

	resp.SetStatusCode(http.StatusBadRequest)
	assert.Equal(http.StatusBadRequest, resp.StatusCode())
}

func TestResponse2(t *testing.T) {
	assert := assert.New(t)
	{
		// when FetchPayload with -1, payload is stream
		resp, err := NewResponse(nil)
		assert.Nil(err)

		err = resp.FetchPayload(-1)
		assert.Nil(err)
		assert.True(resp.IsStream())
		resp.Close()
	}

	{
		// test set payload
		resp, err := NewResponse(nil)
		assert.Nil(err)

		resp.SetPayload(nil)
		assert.Nil(resp.RawPayload())

		resp.SetPayload("")
		assert.Equal(http.NoBody, resp.GetPayload())

		resp.SetPayload("123")
		assert.Equal([]byte("123"), resp.RawPayload())
		assert.Equal(int64(3), resp.PayloadSize())

		assert.Panics(func() { resp.SetPayload(123) })

		resp.SetPayload(strings.NewReader("123"))
		assert.True(resp.IsStream())

		reader := readers.NewByteCountReader(strings.NewReader("123"))
		resp.SetPayload(reader)
		assert.Equal(reader, resp.GetPayload())
	}

	{
		// when FetchPayload with -1, payload is stream
		stdResp := &http.Response{Body: io.NopCloser(strings.NewReader("123"))}
		resp, err := NewResponse(stdResp)
		assert.Nil(err)

		err = resp.FetchPayload(0)
		assert.Nil(err)
		assert.False(resp.IsStream())
		resp.Close()
	}

	{
		// when ContentLength bigger than FetchPayload
		stdResp := &http.Response{Body: io.NopCloser(strings.NewReader("123"))}
		stdResp.ContentLength = 3
		resp, err := NewResponse(stdResp)
		assert.Nil(err)

		err = resp.FetchPayload(1)
		assert.Equal(ErrResponseEntityTooLarge, err)
		resp.Close()
	}

	{
		// when ContentLength is zero
		stdResp := &http.Response{Body: http.NoBody}
		stdResp.ContentLength = 0
		resp, err := NewResponse(stdResp)
		assert.Nil(err)

		err = resp.FetchPayload(100)
		assert.Nil(err)
		resp.Close()
	}

	{
		// unknown context length
		stdResp := &http.Response{Body: io.NopCloser(strings.NewReader("123123123123"))}
		stdResp.ContentLength = -1
		resp, err := NewResponse(stdResp)
		assert.Nil(err)

		err = resp.FetchPayload(100)
		assert.Nil(err)
		resp.Close()
	}

	{
		// unknown context length and actual context length longer than FetchPayload
		stdResp := &http.Response{Body: io.NopCloser(strings.NewReader("123123123123"))}
		stdResp.ContentLength = -1
		resp, err := NewResponse(stdResp)
		assert.Nil(err)

		err = resp.FetchPayload(2)
		assert.Equal(ErrResponseEntityTooLarge, err)
		resp.Close()
	}
}

func TestBuilderResponse(t *testing.T) {
	assert := assert.New(t)
	{
		r := &builderResponse{
			Response: nil,
			rawBody:  []byte("abc"),
		}
		assert.Equal([]byte("abc"), r.RawBody())
		assert.Equal("abc", r.Body())
		_, err := r.JSONBody()
		assert.NotNil(err)

		r = &builderResponse{
			Response: nil,
			rawBody:  []byte("123"),
		}
		_, err = r.JSONBody()
		assert.Nil(err)

		r = &builderResponse{
			Response: nil,
			rawBody:  []byte("{{{{{}"),
		}
		_, err = r.YAMLBody()
		assert.NotNil(err)

		r = &builderResponse{
			Response: nil,
			rawBody:  []byte("123"),
		}
		_, err = r.YAMLBody()
		assert.Nil(err)
	}

	{
		resp := httptest.NewRecorder().Result()
		response, err := NewResponse(resp)
		assert.Nil(err)
		builderResp := response.ToBuilderResponse("DEFAULT").(*builderResponse)
		assert.NotNil(builderResp)
	}
}
