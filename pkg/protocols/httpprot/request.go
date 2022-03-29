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
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/tomasen/realip"
)

// Request wraps http.Request.
type Request struct {
	*http.Request
	getPayload func() io.Reader

	realIP string
}

var _ protocols.Request = (*Request)(nil)

// NewRequest creates a new request from a standard request. The input
// request could be nil, in which case, an empty request is created.
// The caller need to close the body of the input request, if it need
// to be closed.
func NewRequest(req *http.Request) *Request {
	if req == nil {
		req = &http.Request{Body: http.NoBody}
		r := &Request{Request: req}
		r.getPayload = func() io.Reader {
			return http.NoBody
		}
	}

	body := req.Body
	r := &Request{Request: req}
	ra := readers.NewReaderAt(body)
	r.getPayload = func() io.Reader {
		return readers.NewReaderAtReader(ra, 0)
	}
	r.Body = io.NopCloser(r.GetPayload())
	r.realIP = realip.FromRequest(req)

	return r
}

// SetPayload sets the payload of the request to payload.
func (r *Request) SetPayload(payload []byte) {
	r.getPayload = func() io.Reader {
		return bytes.NewReader(payload)
	}
	r.Body = io.NopCloser(r.GetPayload())
}

// GetPayload returns a new payload reader.
// NOTE: Request could be sent more than one time, this is different from
// http.Request, so after a request was sent, the request body should be
// reset for next sent, like below:
//
//    r.Body = io.NopCloser(r.GetPayload())
func (r *Request) GetPayload() io.Reader {
	return r.getPayload()
}

// Clone clones the request and returns the new one.
func (r *Request) Clone() protocols.Request {
	stdr := r.Request.Clone(context.Background())

	r2 := &Request{Request: stdr}
	r2.realIP = r.realIP
	r2.getPayload = r.getPayload
	r2.Body = io.NopCloser(r2.GetPayload())

	return r2
}

// Close closes the request.
func (r *Request) Close() {
}

// HTTPHeader returns the header of the request in type http.Header.
func (r *Request) HTTPHeader() http.Header {
	return r.Std().Header
}

// Header returns the header of the request in type protocols.Header.
func (r *Request) Header() protocols.Header {
	return newHeader(r.HTTPHeader())
}

// Scheme returns the scheme of the request.
func (r *Request) Scheme() string {
	if s := r.Std().URL.Scheme; s != "" {
		return s
	}
	if s := r.HTTPHeader().Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
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
