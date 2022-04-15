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
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/tomasen/realip"
)

// Request wraps http.Request.
type Request struct {
	*http.Request
	payload []byte
	realIP  string
}

var _ protocols.Request = (*Request)(nil)

// NewRequest creates a new request from a standard request.
//
// The body of http.Request can only be read once, but the Request need
// to support being read more times, so we read the full body out here.
// This consumes a lot of memory, but seems no way to avoid it.
func NewRequest(stdr *http.Request) (*Request, error) {
	if stdr == nil {
		stdr = &http.Request{Body: http.NoBody}
		return &Request{Request: stdr}, nil
	}

	var body []byte
	var err error
	if stdr.ContentLength > 0 {
		body = make([]byte, stdr.ContentLength)
		_, err = io.ReadFull(stdr.Body, body)
	} else if stdr.ContentLength == -1 {
		body, err = io.ReadAll(stdr.Body)
	}

	if err != nil {
		return nil, err
	}

	r := &Request{Request: stdr}
	r.realIP = realip.FromRequest(stdr)
	r.SetPayload(body)

	return r, nil
}

// SetPayload sets the payload of the request to payload.
func (r *Request) SetPayload(payload []byte) {
	r.payload = payload
	r.Body = io.NopCloser(r.GetPayload())
}

// GetPayload returns a new payload reader.
func (r *Request) GetPayload() io.Reader {
	if len(r.payload) == 0 {
		return http.NoBody
	} else {
		return bytes.NewReader(r.payload)
	}
}

// RawPayload returns the payload in []byte, the caller should
// not modify its content.
func (r *Request) RawPayload() []byte {
	return r.payload
}

// Close closes the request.
func (r *Request) Close() {
}

// MetaSize returns the meta data size of the request.
func (r *Request) MetaSize() int {
	// Reference: https://tools.ietf.org/html/rfc2616#section-5
	//
	// meta length is the length of:
	// w.stdr.Method + " "
	// + stdr.URL.RequestURI() + " "
	// + stdr.Proto + "\r\n",
	// + w.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(r.Method()) + 1
	size += len(r.Std().URL.RequestURI()) + 1
	size += len(r.Proto()) + 2

	lines := 0
	for key, values := range r.HTTPHeader() {
		for _, value := range values {
			lines++
			size += len(key) + len(value)
		}
	}

	size += lines * 2 // ": "
	if lines > 1 {
		size += (lines - 1) * 2 // "\r\n"
	}

	return size + 4
}

// HTTPHeader returns the header of the request in type http.Header.
func (r *Request) HTTPHeader() http.Header {
	return r.Std().Header
}

// Header returns the header of the request in type protocols.Header.
func (r *Request) Header() protocols.Header {
	return newHeader(r.HTTPHeader())
}

// RequestScheme returns the scheme of the request.
func RequestScheme(r *http.Request) string {
	if s := r.URL.Scheme; s != "" {
		return s
	}
	if s := r.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// Scheme returns the scheme of the request.
func (r *Request) Scheme() string {
	return RequestScheme(r.Std())
}

// RealIP returns the real IP of the request.
// TODO: if a request is cloned and modified, RealIP maybe wrong.
func (r *Request) RealIP() string {
	return r.realIP
}

// Std returns the underlying http.Request.
func (r *Request) Std() *http.Request {
	return r.Request
}

// URL returns url of the request.
func (r *Request) URL() *url.URL {
	return r.Std().URL
}

// Proto returns proto of the request.
func (r *Request) Proto() string {
	return r.Std().Proto
}

// Method returns method of the request.
func (r *Request) Method() string {
	return r.Std().Method
}

// Cookie returns the named cookie.
func (r *Request) Cookie(name string) (*http.Cookie, error) {
	return r.Std().Cookie(name)
}

// Cookies returns all cookies.
func (r *Request) Cookies() []*http.Cookie {
	return r.Std().Cookies()
}

// AddCookie add a cookie to the request.
func (r *Request) AddCookie(cookie *http.Cookie) {
	r.Std().AddCookie(cookie)
}

// Context returns the request context.
func (r *Request) Context() context.Context {
	return r.Std().Context()
}

// SetMethod sets the request method.
func (r *Request) SetMethod(method string) {
	r.Std().Method = method
}

// Host returns host of the request.
func (r *Request) Host() string {
	return r.Std().Host
}

// SetHost sets host.
func (r *Request) SetHost(host string) {
	r.Std().Host = host
}

// Path returns path.
func (r *Request) Path() string {
	return r.Std().URL.Path
}

// SetPath sets path of the request.
func (r *Request) SetPath(path string) {
	r.Std().URL.Path = path
}
