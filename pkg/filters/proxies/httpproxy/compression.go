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
	"strings"

	"github.com/megaease/easegress/v2/pkg/util/readers"
)

// TODO: Expose more options: compression level, mime types.

type (
	// compression is filter compression.
	compression struct {
		spec *CompressionSpec
	}

	// CompressionSpec describes the compression.
	CompressionSpec struct {
		MinLength uint32 `json:"minLength"`
	}
)

const (
	keyAcceptEncoding  = "Accept-Encoding"
	keyContentEncoding = "Content-Encoding"
	keyContentLength   = "Content-Length"
	keyVary            = "Vary"
)

func newCompression(spec *CompressionSpec) *compression {
	return &compression{
		spec: spec,
	}
}

func (c *compression) compress(req *http.Request, resp *http.Response) bool {
	if !c.acceptGzip(req) {
		return false
	}

	if c.alreadyGziped(resp) {
		return false
	}

	if resp.ContentLength != -1 && resp.ContentLength < int64(c.spec.MinLength) {
		return false
	}

	resp.ContentLength = -1
	resp.Header.Del(keyContentLength)
	resp.Header.Set(keyContentEncoding, "gzip")
	resp.Header.Add(keyVary, keyContentEncoding)

	resp.Body = readers.NewGZipCompressReader(resp.Body)
	return true
}

func (c *compression) alreadyGziped(resp *http.Response) bool {
	for _, ce := range resp.Header.Values(keyContentEncoding) {
		if strings.Contains(ce, "gzip") {
			return true
		}
	}

	return false
}

func (c *compression) acceptGzip(req *http.Request) bool {
	acceptEncodings := req.Header.Values(keyAcceptEncoding)

	// NOTE: Easegress does not support parsing qvalue for performance.
	// Reference: https://tools.ietf.org/html/rfc2616#section-14.3
	if len(acceptEncodings) > 0 {
		for _, ae := range acceptEncodings {
			if strings.Contains(ae, "*/*") ||
				strings.Contains(ae, "gzip") {
				return true
			}
		}
		return false
	}

	return true
}
