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

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/readers"
)

func init() {
	protocols.Register("http", &Protocol{})
}

type (
	header struct {
		header http.Header
	}

	payload struct {
		readerAt *readers.ReaderAt
	}
)

var _ protocols.Header = (*header)(nil)

func newHeader(h http.Header) *header {
	return &header{header: h}
}

func (h *header) Add(key, value string) {
	h.header.Add(key, value)
}

func (h *header) Set(key, value string) {
	h.header.Set(key, value)
}

func (h *header) Get(key string) string {
	return h.header.Get(key)
}

func (h *header) Values(key string) []string {
	return h.header.Values(key)
}

func (h *header) Del(key string) {
	h.header.Del(key)
}

func (h *header) Clone() protocols.Header {
	return &header{header: h.header.Clone()}
}

func (h *header) Iter(f func(key string, values []string)) {
	for k, vs := range h.header {
		f(k, vs)
	}
}

var _ protocols.Payload = (*payload)(nil)

func newPayload(r io.Reader) *payload {
	if r != nil {
		return &payload{
			readerAt: readers.NewReaderAt(r),
		}
	}
	return &payload{}
}

func (p *payload) NewReader() io.Reader {
	if p.readerAt != nil {
		return readers.NewReaderAtReader(p.readerAt, 0)
	}
	return nil
}

func (p *payload) SetReader(reader io.Reader, closePreviousReader bool) {
	if closePreviousReader {
		if p.readerAt != nil {
			p.readerAt.Close()
		}
	}
	p.readerAt = readers.NewReaderAt(reader)
}

func (p *payload) Close() {
	if p.readerAt != nil {
		p.readerAt.Close()
	}
}

type Protocol struct {
}

var _ protocols.Protocol = (*Protocol)(nil)

func (p *Protocol) CreateRequest(req interface{}) protocols.Request {
	r := req.(*http.Request)
	return NewRequest(r)
}

func (p *Protocol) CreateResponse(resp interface{}) protocols.Response {
	w := resp.(http.ResponseWriter)
	return NewResponse(w)
}

func (p *Protocol) CreateLoadBalancer(lb string, servers []protocols.Server) (protocols.LoadBalancer, error) {
	return nil, nil
}

func (p *Protocol) CreateServer(uri string) (protocols.Server, error) {
	return nil, nil
}

func (p *Protocol) CreateTrafficMatcher(spec interface{}) (protocols.TrafficMatcher, error) {
	return nil, nil
}

type Server struct {
	URL            string   `yaml:"url" jsonschema:"required,format=url"`
	Tags           []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
	W              int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	addrIsHostName bool
}

func (s *Server) Weight() int {
	return s.W
}

func (s *Server) SendRequest(req protocols.Request) (protocols.Response, error) {
	req = req.Clone()

	return nil, nil
}

func GetHTTPRequestAndResponse(ctx context.Context) (*Request, *Response) {
	req := ctx.Request().(*Request)
	resp := ctx.Response().(*Response)
	return req, resp
}
