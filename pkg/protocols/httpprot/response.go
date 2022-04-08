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
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/readers"
)

// Response wraps http.Response.
type Response struct {
	// TODO: we only need StatusCode, Header and Body, that's can avoid
	// using the big http.Response object.
	*http.Response
	getPayload func() io.Reader
}

var _ protocols.Response = (*Response)(nil)

// NewResponse creates a new response from a standard response. The input
// response could be nil, in which case, an empty response is created.
// The caller need to close the body of the input response, if it need
// to be closed.
func NewResponse(resp *http.Response) *Response {
	if resp == nil {
		return &Response{
			Response: &http.Response{
				Body:       http.NoBody,
				StatusCode: http.StatusOK,
			},
			getPayload: func() io.Reader {
				return http.NoBody
			},
		}
	}

	ra := readers.NewReaderAt(resp.Body)
	r := &Response{Response: resp}
	r.getPayload = func() io.Reader {
		return readers.NewReaderAtReader(ra, 0)
	}
	r.Body = io.NopCloser(r.GetPayload())

	return r
}

// Std returns the underlying http.Response.
func (r *Response) Std() *http.Response {
	return r.Response
}

// MetaSize returns the meta data size of the response.
func (r *Response) MetaSize() int {
	stdr := r.Std()
	text := http.StatusText(stdr.StatusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(stdr.StatusCode)
	}

	// meta length is the length of:
	// resp.Proto + " "
	// + strconv.Itoa(resp.StatusCode) + " "
	// + text + "\r\n",
	// + resp.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(stdr.Proto) + 1
	if stdr.StatusCode >= 100 && stdr.StatusCode < 1000 {
		size += 3 + 1
	} else {
		size += len(strconv.Itoa(stdr.StatusCode)) + 1
	}
	size += len(text) + 2

	lines := 0
	for key, values := range stdr.Header {
		for _, value := range values {
			lines++
			size += len(key) + len(value)
		}
	}

	size += lines * 2 // ": "
	if lines > 1 {
		size += (lines - 1) * 2 // "\r\n"
	}

	return size
}

// StatusCode returns the status code of the response.
func (r *Response) StatusCode() int {
	return r.Std().StatusCode
}

// SetStatusCode sets the status code of the response.
func (r *Response) SetStatusCode(code int) {
	r.Std().StatusCode = code
}

// SetCookie adds a Set-Cookie header to the response's headers.
func (r *Response) SetCookie(cookie *http.Cookie) {
	if v := cookie.String(); v != "" {
		r.HTTPHeader().Add("Set-Cookie", v)
	}
}

// SetPayload sets the payload of the response to payload.
func (r *Response) SetPayload(payload []byte) {
	r.getPayload = func() io.Reader {
		return bytes.NewReader(payload)
	}
	r.Std().Body = io.NopCloser(r.GetPayload())
}

// GetPayload returns a new payload reader.
func (r *Response) GetPayload() io.Reader {
	return r.getPayload()
}

// HTTPHeader returns the header of the response in type http.Header.
func (r *Response) HTTPHeader() http.Header {
	return r.Std().Header
}

// Header returns the header of the response in type protocols.Header.
func (r *Response) Header() protocols.Header {
	return newHeader(r.HTTPHeader())
}

// Clone clones the response and returns the new one.
func (r *Response) Clone() protocols.Response {
	return &Response{
		Response: &http.Response{
			StatusCode: r.StatusCode(),
			Header:     r.HTTPHeader().Clone(),
			Body:       io.NopCloser(r.GetPayload()),
		},
		getPayload: r.getPayload,
	}
}

// Close closes the response.
func (r *Response) Close() {
}
