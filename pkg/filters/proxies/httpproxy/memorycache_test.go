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
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestMemoryCache(t *testing.T) {
	assert := assert.New(t)

	mc := NewMemoryCache(&MemoryCacheSpec{})
	assert.NotNil(mc)

	mc = NewMemoryCache(&MemoryCacheSpec{
		Expiration:    "1m",
		MaxEntryBytes: 10,
		Methods:       []string{http.MethodGet},
		Codes:         []int{http.StatusOK},
	})
	assert.NotNil(mc)

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)
	resp, _ := httpprot.NewResponse(nil)

	// Too big payload
	resp.SetPayload([]byte("0123456789A"))
	mc.Store(req, resp)
	assert.Nil(mc.Load(req))

	// Wrong method
	resp.SetPayload([]byte("0123456789"))
	req.Std().Method = http.MethodPut
	mc.Store(req, resp)
	assert.Nil(mc.Load(req))

	// Wrong status code
	req.Std().Method = http.MethodGet
	resp.SetStatusCode(http.StatusNotFound)
	mc.Store(req, resp)
	assert.Nil(mc.Load(req))

	// Request cache control
	resp.SetStatusCode(http.StatusOK)
	req.HTTPHeader().Set(keyCacheControl, "no-cache")
	mc.Store(req, resp)
	assert.Nil(mc.Load(req))

	// Response cache control
	req.HTTPHeader().Del(keyCacheControl)
	resp.HTTPHeader().Set(keyCacheControl, "no-cache")
	mc.Store(req, resp)
	assert.Nil(mc.Load(req))

	// Success
	resp.HTTPHeader().Del(keyCacheControl)
	mc.Store(req, resp)
	assert.NotNil(mc.Load(req))
}
