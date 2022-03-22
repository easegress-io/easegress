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
	"io"
	"net/http"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
)

type (
	// Response provide following methods
	// 	protocols.Response
	// 	Std() http.ResponseWriter
	// 	StatusCode() int
	// 	SetStatusCode(code int)
	// 	SetCookie(cookie *http.Cookie)
	// 	FlushedBodyBytes() uint64
	// 	OnFlushBody(fn BodyFlushFunc)

	Response struct {
		std         http.ResponseWriter
		code        int
		bodyWritten uint64

		header         *header
		payload        *payload
		bodyFlushFuncs []BodyFlushFunc
	}

	BodyFlushFunc = func(body []byte, complete bool) (newBody []byte)
)

var _ protocols.Response = (*Response)(nil)

func newResponse(w http.ResponseWriter) *Response {
	return &Response{
		std:    w,
		code:   http.StatusOK,
		header: newHeader(w.Header()),
	}
}

func (resp *Response) Std() http.ResponseWriter {
	return resp.std
}

func (resp *Response) StatusCode() int {
	return resp.code
}

func (resp *Response) SetStatusCode(code int) {
	resp.code = code
}

func (resp *Response) SetCookie(cookie *http.Cookie) {
	http.SetCookie(resp.std, cookie)
}

func (resp *Response) Payload() protocols.Payload {
	return resp.payload
}

func (resp *Response) Header() protocols.Header {
	return resp.header
}

func (resp *Response) FlushedBodyBytes() uint64 {
	return resp.bodyWritten
}

func (resp *Response) Finish() {
	resp.std.WriteHeader(resp.StatusCode())
	reader := resp.payload.NewReader()
	if reader == nil {
		return
	}

	defer func() {
		resp.payload.Close()
	}()

	written, err := io.Copy(resp.std, resp.payload.NewReader())
	if err != nil {
		logger.Warnf("copy body failed: %v", err)
		return
	}
	resp.bodyWritten += uint64(written)
}

func (resp *Response) Clone() protocols.Response {
	return nil
}

func (resp *Response) OnFlushBody(fn BodyFlushFunc) {
	resp.bodyFlushFuncs = append(resp.bodyFlushFuncs, fn)
}
