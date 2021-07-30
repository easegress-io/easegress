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

package contexttest

import (
	"io"
	"net/http"

	"github.com/megaease/easegress/pkg/util/httpheader"
)

type MockedHTTPRequest struct {
	MockedRealIP      func() string
	MockedMethod      func() string
	MockedSetMethod   func(method string)
	MockedScheme      func() string
	MockedHost        func() string
	MockedSetHost     func(host string)
	MockedPath        func() string
	MockedSetPath     func(path string)
	MockedEscapedPath func() string
	MockedQuery       func() string
	MockedSetQuery    func(query string)
	MockedFragment    func() string
	MockedProto       func() string
	MockedHeader      func() *httpheader.HTTPHeader
	MockedCookie      func(name string) (*http.Cookie, error)
	MockedCookies     func() []*http.Cookie
	MockedAddCookie   func(cookie *http.Cookie)
	MockedBody        func() io.Reader
	MockedSetBody     func(io.Reader)
	MockedStd         func() *http.Request
	MockedSize        func() uint64
}

func (r *MockedHTTPRequest) RealIP() string {
	if r.MockedRealIP != nil {
		return r.MockedRealIP()
	}
	return ""
}

func (r *MockedHTTPRequest) Method() string {
	if r.MockedMethod != nil {
		return r.MockedMethod()
	}
	return ""
}

func (r *MockedHTTPRequest) SetMethod(method string) {
	if r.MockedSetMethod != nil {
		r.MockedSetMethod(method)
	}
}

func (r *MockedHTTPRequest) Scheme() string {
	if r.MockedScheme != nil {
		return r.MockedScheme()
	}
	return ""
}

func (r *MockedHTTPRequest) Host() string {
	if r.MockedHost != nil {
		return r.MockedHost()
	}
	return ""
}

func (r *MockedHTTPRequest) SetHost(host string) {
	if r.MockedSetHost != nil {
		r.MockedSetHost(host)
	}
}

func (r *MockedHTTPRequest) Path() string {
	if r.MockedPath != nil {
		return r.MockedPath()
	}
	return ""
}

func (r *MockedHTTPRequest) SetPath(path string) {
	if r.MockedSetPath != nil {
		r.MockedSetPath(path)
	}
}

func (r *MockedHTTPRequest) EscapedPath() string {
	if r.MockedEscapedPath != nil {
		return r.MockedEscapedPath()
	}
	return ""
}

func (r *MockedHTTPRequest) Query() string {
	if r.MockedQuery != nil {
		return r.MockedQuery()
	}
	return ""
}

func (r *MockedHTTPRequest) SetQuery(query string) {
	if r.MockedSetQuery != nil {
		r.MockedSetQuery(query)
	}
}

func (r *MockedHTTPRequest) Fragment() string {
	if r.MockedFragment != nil {
		return r.MockedFragment()
	}
	return ""
}

func (r *MockedHTTPRequest) Proto() string {
	if r.MockedProto != nil {
		return r.MockedProto()
	}
	return ""
}

func (r *MockedHTTPRequest) Header() *httpheader.HTTPHeader {
	if r.MockedHeader != nil {
		return r.MockedHeader()
	}
	return nil
}

func (r *MockedHTTPRequest) Cookie(name string) (*http.Cookie, error) {
	if r.MockedCookie != nil {
		return r.MockedCookie(name)
	}
	return nil, nil
}

func (r *MockedHTTPRequest) Cookies() []*http.Cookie {
	if r.MockedCookies != nil {
		return r.MockedCookies()
	}
	return nil
}

func (r *MockedHTTPRequest) AddCookie(cookie *http.Cookie) {
	if r.MockedAddCookie != nil {
		r.MockedAddCookie(cookie)
	}
}

func (r *MockedHTTPRequest) Body() io.Reader {
	if r.MockedBody != nil {
		return r.MockedBody()
	}
	return nil
}

func (r *MockedHTTPRequest) SetBody(body io.Reader) {
	if r.MockedSetBody != nil {
		r.MockedSetBody(body)
	}
}

func (r *MockedHTTPRequest) Std() *http.Request {
	if r.MockedStd != nil {
		return r.MockedStd()
	}
	return &http.Request{}
}

func (r *MockedHTTPRequest) Size() uint64 {
	if r.MockedSize != nil {
		return r.MockedSize()
	}
	return 0
}
