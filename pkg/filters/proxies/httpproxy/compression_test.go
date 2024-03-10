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
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAcceptGzip(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	req, _ := http.NewRequest(http.MethodGet, "https://megaease.com", nil)
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}

	req.Header.Add(keyAcceptEncoding, "text/text")
	if c.acceptGzip(req) {
		t.Error("accept gzip should be false")
	}

	req.Header.Add(keyAcceptEncoding, "*/*")
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}

	req.Header.Del(keyAcceptEncoding)
	req.Header.Add(keyAcceptEncoding, "gzip")
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}
}

func TestAlreadyGziped(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	resp := &http.Response{Header: http.Header{}}

	if c.alreadyGziped(resp) {
		t.Error("already gziped should be false")
	}

	resp.Header.Add(keyContentEncoding, "text")
	if c.alreadyGziped(resp) {
		t.Error("already gziped should be false")
	}

	resp.Header.Add(keyContentEncoding, "gzip")
	if !c.alreadyGziped(resp) {
		t.Error("already gziped should be true")
	}
}

func TestCompress(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	req, _ := http.NewRequest(http.MethodGet, "https://megaease.com", nil)
	resp := &http.Response{Header: http.Header{}}

	rawBody := strings.Repeat("this is the raw body. ", 100)
	resp.Body = io.NopCloser(strings.NewReader(rawBody))

	resp.ContentLength = 20
	c.compress(req, resp)
	if resp.Header.Get(keyContentEncoding) == "gzip" {
		t.Error("body should not be gziped")
	}

	resp.Body = http.NoBody

	resp.ContentLength = 120
	c.compress(req, resp)
	if resp.Header.Get(keyContentEncoding) != "gzip" {
		t.Error("body should be gziped")
	}

	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		t.Error("data length should not be zero")
	}
}
