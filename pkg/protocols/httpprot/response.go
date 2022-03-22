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
	Response interface {
		protocols.Response

		Std() http.ResponseWriter

		StatusCode() int
		SetStatusCode(code int)
		SetCookie(cookie *http.Cookie)
		FlushedBodyBytes() uint64
		OnFlushBody(fn BodyFlushFunc)
	}

	response struct {
		std         http.ResponseWriter
		code        int
		bodyWritten uint64

		header         *header
		payload        *payload
		bodyFlushFuncs []BodyFlushFunc
	}

	BodyFlushFunc = func(body []byte, complete bool) (newBody []byte)
)

var _ Response = (*response)(nil)
var _ protocols.Response = (*response)(nil)

func newResponse(w http.ResponseWriter) Response {
	return &response{
		std:    w,
		code:   http.StatusOK,
		header: newHeader(w.Header()),
	}
}

func (resp *response) Std() http.ResponseWriter {
	return resp.std
}

func (resp *response) StatusCode() int {
	return resp.code
}

func (resp *response) SetStatusCode(code int) {
	resp.code = code
}

func (resp *response) SetCookie(cookie *http.Cookie) {
	http.SetCookie(resp.std, cookie)
}

func (resp *response) Payload() protocols.Payload {
	return resp.payload
}

func (resp *response) Header() protocols.Header {
	return resp.header
}

func (resp *response) FlushedBodyBytes() uint64 {
	return resp.bodyWritten
}

func (resp *response) Finish() {
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

func (resp *response) Clone() protocols.Response {
	return nil
}

func (resp *response) OnFlushBody(fn BodyFlushFunc) {
	resp.bodyFlushFuncs = append(resp.bodyFlushFuncs, fn)
}
