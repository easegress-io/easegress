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

package grpcprot

import (
	"github.com/megaease/easegress/pkg/protocols"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
)

// Response wrapper status.Status
type Response struct {
	// instead status.Status
	// nil or from codes.OK means success, otherwise means fail
	*status.Status
	header  *Header
	trailer *Trailer
}

var (
	_ protocols.Response = (*Response)(nil)
)

func NewResponse() *Response {
	return &Response{
		Status:  nil,
		header:  NewHeader(metadata.New(nil)),
		trailer: NewTrailer(metadata.New(nil)),
	}
}

func (r *Response) SetTrailer(trailer *Trailer) {
	r.trailer.md = trailer.md
}

func (r *Response) Trailer() protocols.Trailer {
	return r.trailer
}

func (r *Response) RawTrailer() *Trailer {
	return r.trailer
}

func (r *Response) GetStatus() *status.Status {
	return r.Status
}

func (r *Response) SetStatus(s *status.Status) {
	if s == nil {
		r.Status = status.New(codes.OK, "OK")
		return
	}
	r.Status = s
}

func (r *Response) StatusCode() int {
	if r.Status == nil {
		return int(codes.OK)
	}
	return int(r.Status.Code())
}

func (r *Response) SetHeader(header *Header) {
	r.header.md = header.md
}

func (r *Response) Header() protocols.Header {
	return r.header
}

// RawHeader returns the header of the request in type metadata.MD.
func (r *Response) RawHeader() *Header {
	return r.header
}

func (r *Response) IsStream() bool {
	return true
}

func (r *Response) SetPayload(payload interface{}) {
	panic("implement me")
}

func (r *Response) GetPayload() io.Reader {
	panic("implement me")
}

func (r *Response) RawPayload() []byte {
	panic("implement me")
}

func (r *Response) PayloadSize() int64 {
	panic("implement me")
}

func (r *Response) ToBuilderResponse(name string) interface{} {
	panic("implement me")
}

func (r *Response) Close() {

}
