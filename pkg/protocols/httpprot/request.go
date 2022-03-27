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
	realIP string
}

var _ protocols.Request = (*Request)(nil)

// NewRequest creates a new request from a standard request, the input
// request must be request from the Go HTTP package or nil.
func NewRequest(r *http.Request) *Request {
	if r == nil {
		r = &http.Request{Body: http.NoBody}
		r.GetBody = func() (io.ReadCloser, error) {
			return http.NoBody, nil
		}
		return &Request{Request: r}
	}

	// The body of http.Request can only be read once, but we need our
	// payload to support repeatable read, and the Clone of our request
	// will also duplicate the payload. But we cannot do it directly by
	// changing the body of the input http.Request, because it is an
	// io.Closer, if we changes it, we don't know when to Close it.
	//
	// we create a new request with a new body to avoid using the input
	// request, so that the Go HTTP package could close the body.
	//
	// use Clone instead of WithContext??
	r = r.WithContext(context.Background())
	req := &Request{Request: r}
	ra := readers.NewReaderAt(r.Body)
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(readers.NewReaderAtReader(ra, 0)), nil
	}
	r.Body = io.NopCloser(req.GetPayload())
	req.realIP = realip.FromRequest(r)

	return req
}

// SetPayload sets the payload of the request to payload.
func (r *Request) SetPayload(payload []byte) {
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	r.Body = io.NopCloser(r.GetPayload())
}

// GetPayload returns a new payload reader.
func (r *Request) GetPayload() io.Reader {
	p, _ := r.GetBody()
	return p
}

// Clone clones the request and returns the new one.
func (r *Request) Clone() protocols.Request {
	stdr := r.Request.Clone(context.Background())

	req := &Request{Request: stdr}
	req.realIP = r.realIP
	req.Body = io.NopCloser(req.GetPayload())

	return req
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
