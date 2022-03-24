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
	"os"

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
var bodyFlushBuffSize = 8 * int64(os.Getpagesize())

// NewResponse creates a new response from a standard response writer.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{
		std:            w,
		code:           http.StatusOK,
		header:         newHeader(w.Header()),
		payload:        newPayload(nil),
		bodyFlushFuncs: []BodyFlushFunc{},
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
	defer resp.Payload().Close()

	copyToClient := func(src io.Reader) (succeed bool) {
		written, err := io.Copy(resp.std, src)
		if err != nil {
			logger.Warnf("copy body failed: %v", err)
			return false
		}
		resp.bodyWritten += uint64(written)
		return true
	}

	if len(resp.bodyFlushFuncs) == 0 {
		copyToClient(reader)
		return
	}

	buff := bytes.NewBuffer(nil)
	for {
		buff.Reset()
		_, err := io.CopyN(buff, reader, bodyFlushBuffSize)
		body := buff.Bytes()

		switch err {
		case nil:
			for _, fn := range resp.bodyFlushFuncs {
				body = fn(body, false)
			}
			if !copyToClient(bytes.NewReader(body)) {
				return
			}
		case io.EOF:
			for _, fn := range resp.bodyFlushFuncs {
				body = fn(body, true)
			}

			copyToClient(bytes.NewReader(body))
			return
		default:
			resp.std.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

}

func (resp *Response) OnFlushBody(fn BodyFlushFunc) {
	resp.bodyFlushFuncs = append(resp.bodyFlushFuncs, fn)
}
