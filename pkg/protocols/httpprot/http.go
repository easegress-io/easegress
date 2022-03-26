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
	"net/http"

	"github.com/megaease/easegress/pkg/protocols"
)

func init() {
	protocols.Register("http", &Protocol{})
}

// Header wraps the http header.
type Header struct {
	http.Header
}

func newHeader(h http.Header) *Header {
	return &Header{Header: h}
}

// Add adds the key value pair to the header.
func (h *Header) Add(key string, value interface{}) {
	h.Header.Add(key, value.(string))
}

// Set sets the header entries associated with key to value.
func (h *Header) Set(key string, value interface{}) {
	h.Header.Set(key, value.(string))
}

// Get gets the first value associated with the given key.
func (h *Header) Get(key string) interface{} {
	return h.Header.Get(key)
}

// Del deletes the values associated with key.
func (h *Header) Del(key string) {
	h.Header.Del(key)
}

// Clone returns a copy of h.
func (h *Header) Clone() protocols.Header {
	return &Header{Header: h.Header.Clone()}
}

// Walk calls fn for each key value pair of the header.
func (h *Header) Walk(fn func(key string, value interface{}) bool) {
	for k, vs := range h.Header {
		if !fn(k, vs) {
			break
		}
	}
}

/*
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
*/
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
