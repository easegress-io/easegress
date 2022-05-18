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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilResponse(t *testing.T) {
	assert := assert.New(t)

	resp, err := NewResponse(nil)
	assert.Nil(err)
	defer resp.Close()
	resp.HTTPHeader().Set("foo", "bar")

	l, err := resp.FetchPayload()
	assert.Nil(err)
	assert.Equal(0, l)

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

	l, err = resp.FetchPayload()
	assert.Nil(err)
	assert.Equal(5, l)

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
