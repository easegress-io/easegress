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
	"net/http"
	"net/url"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/tomasen/realip"
)

type (
	Request interface {
		protocols.Request

		Std() *http.Request
		URL() *url.URL
		Path() string
		SetPath(path string)
		Scheme() string

		RealIP() string
		Proto() string
		Method() string
		SetMethod(method string)
		Host() string
		SetHost(host string)

		Cookie(name string) (*http.Cookie, error)
		Cookies() []*http.Cookie
		AddCookie(cookie *http.Cookie)
	}

	request struct {
		std    *http.Request
		realIP string

		header  *header
		payload *payload
	}
)

var _ Request = (*request)(nil)
var _ protocols.Request = (*request)(nil)

func newRequest(r *http.Request) Request {
	req := &request{}
	req.std = r
	req.realIP = realip.FromRequest(r)
	req.payload = newPayload(r.Body)
	req.header = newHeader(r.Header)
	return req
}

func (r *request) Std() *http.Request {
	return r.std
}

func (r *request) URL() *url.URL {
	return r.std.URL
}

func (r *request) RealIP() string {
	return r.realIP
}

func (r *request) Proto() string {
	return r.std.Proto
}

func (r *request) Method() string {
	return r.std.Method
}

func (r *request) Cookie(name string) (*http.Cookie, error) {
	return r.std.Cookie(name)
}

func (r *request) Cookies() []*http.Cookie {
	return r.std.Cookies()
}

func (r *request) AddCookie(cookie *http.Cookie) {
	r.std.AddCookie(cookie)
}

func (r *request) Header() protocols.Header {
	return r.header
}

func (r *request) Payload() protocols.Payload {
	return r.payload
}

func (r *request) Context() context.Context {
	return r.std.Context()
}

func (r *request) WithContext(ctx context.Context) {
	r.std = r.std.WithContext(ctx)
}

func (r *request) Finish() {
	r.payload.Close()
}

func (r *request) SetMethod(method string) {
	r.std.Method = method
}

func (r *request) Host() string {
	return r.std.Host
}

func (r *request) SetHost(host string) {
	r.std.Host = host
}

func (r *request) Clone() protocols.Request {
	return nil
}

func (r *request) Path() string {
	return r.std.URL.Path
}

func (r *request) SetPath(path string) {
	r.std.URL.Path = path
}

func (r *request) Scheme() string {
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
