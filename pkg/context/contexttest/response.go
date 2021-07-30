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
	"net/http/httptest"

	"github.com/megaease/easegress/pkg/util/httpheader"
)

type MockedHTTPResponse struct {
	MockedStatusCode    func() int
	MockedSetStatusCode func(code int)
	MockedHeader        func() *httpheader.HTTPHeader
	MockedSetCookie     func(cookie *http.Cookie)
	MockedSetBody       func(body io.Reader)
	MockedBody          func() io.Reader
	MockedOnFlushBody   func(func(body []byte, complete bool) (newBody []byte))
	MockedStd           func() http.ResponseWriter
	MockedSize          func() uint64
}

func (r *MockedHTTPResponse) StatusCode() int {
	if r.MockedStatusCode != nil {
		return r.MockedStatusCode()
	}
	return 0
}

func (r *MockedHTTPResponse) SetStatusCode(code int) {
	if r.MockedSetStatusCode != nil {
		r.MockedSetStatusCode(code)
	}
}

func (r *MockedHTTPResponse) Header() *httpheader.HTTPHeader {
	if r.MockedHeader != nil {
		return r.MockedHeader()
	}
	return nil
}

func (r *MockedHTTPResponse) SetCookie(cookie *http.Cookie) {
	if r.MockedSetCookie != nil {
		r.MockedSetCookie(cookie)
	}
}

func (r *MockedHTTPResponse) SetBody(body io.Reader) {
	if r.MockedSetBody != nil {
		r.MockedSetBody(body)
	}
}

func (r *MockedHTTPResponse) Body() io.Reader {
	if r.MockedBody != nil {
		return r.MockedBody()
	}
	return nil
}

func (r *MockedHTTPResponse) OnFlushBody(fn func(body []byte, complete bool) (newBody []byte)) {
	if r.MockedOnFlushBody != nil {
		r.MockedOnFlushBody(fn)
	}
}

func (r *MockedHTTPResponse) Std() http.ResponseWriter {
	if r.MockedStd != nil {
		return r.MockedStd()
	}
	return &httptest.ResponseRecorder{}
}

func (r *MockedHTTPResponse) Size() uint64 {
	if r.MockedSize != nil {
		return r.MockedSize()
	}
	return 0
}
