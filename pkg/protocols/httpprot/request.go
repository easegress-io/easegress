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
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/tomasen/realip"
)

type (
	// Request provide following methods
	// 	protocols.Request
	// 	Std() *http.Request
	// 	URL() *url.URL
	// 	Path() string
	// 	SetPath(path string)
	// 	Scheme() string
	// 	RealIP() string
	// 	Proto() string
	// 	Method() string
	// 	SetMethod(method string)
	// 	Host() string
	// 	SetHost(host string)
	// 	Cookie(name string) (*http.Cookie, error)
	// 	Cookies() []*http.Cookie
	// 	AddCookie(cookie *http.Cookie)
	Request struct {
		std    *http.Request
		realIP string

		header  *Header
		payload io.ReaderAt
	}
)

var _ protocols.Request = (*Request)(nil)

// NewRequest creates a new request from a standard request.
func NewRequest(r *http.Request) *Request {
	req := &Request{}
	req.std = r
	req.realIP = realip.FromRequest(r)
	req.header = newHeader(r.Header)

	req.payload = readers.NewReaderAt(r.Body)
	req.std.Body = io.NopCloser(readers.NewReaderAtReader(req.payload, 0))
	return req
}

func (r *Request) Std() *http.Request {
	return r.std
}

func (r *Request) URL() *url.URL {
	return r.std.URL
}

func (r *Request) RealIP() string {
	return r.realIP
}

func (r *Request) Proto() string {
	return r.std.Proto
}

func (r *Request) Method() string {
	return r.std.Method
}

func (r *Request) Cookie(name string) (*http.Cookie, error) {
	return r.std.Cookie(name)
}

func (r *Request) Cookies() []*http.Cookie {
	return r.std.Cookies()
}

func (r *Request) AddCookie(cookie *http.Cookie) {
	r.std.AddCookie(cookie)
}

func (r *Request) HTTPHeader() http.Header {
	return r.std.Header
}

func (r *Request) Header() protocols.Header {
	return r.header
}

func (r *Request) SetPayload(reader io.Reader) {
	r.payload = readers.NewReaderAt(reader)
	r.std.Body = io.NopCloser(readers.NewReaderAtReader(r.payload, 0))
}

func (r *Request) GetPayloadReader() io.Reader {
	return readers.NewReaderAtReader(r.payload, 0)
}

func (r *Request) Context() context.Context {
	return r.std.Context()
}

func (r *Request) WithContext(ctx context.Context) {
	r.std = r.std.WithContext(ctx)
}

func (r *Request) Finish() {
	// r.payload.Close()
}

func (r *Request) SetMethod(method string) {
	r.std.Method = method
}

func (r *Request) Host() string {
	return r.std.Host
}

func (r *Request) SetHost(host string) {
	r.std.Host = host
}

func (r *Request) Clone() protocols.Request {
	req := r.std.Clone(context.Background())
	// TODO: CLONE
	// req.Body = io.NopCloser(r.payload.NewReader())
	return NewRequest(req)
}

func (r *Request) Path() string {
	return r.std.URL.Path
}

func (r *Request) SetPath(path string) {
	r.std.URL.Path = path
}

func (r *Request) Scheme() string {
	if s := r.std.URL.Scheme; s != "" {
		return s
	}
	if s := r.std.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	if r.std.TLS != nil {
		return "https"
	}
	return "http"
}
