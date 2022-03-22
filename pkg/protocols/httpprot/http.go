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

	"github.com/megaease/easegress/pkg/protocols"
)

func init() {
	protocols.Register("http", &Protocol{})
}

type (
	header struct {
		header http.Header
	}

	payload struct {
		reader io.Reader
		data   []byte
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
	data, err := io.ReadAll(r)
	// here how to deal with readall error??????
	// return nil when error happens????
	if err != nil {
		panic(err)
	}
	return &payload{reader: r, data: data}
}

func (p *payload) NewReader() io.Reader {
	return bytes.NewReader(p.data)
}

func (p *payload) SetReader(reader io.Reader, closePreviousReader bool) {
	if closePreviousReader {
		if closer, ok := p.reader.(io.Closer); ok {
			closer.Close()
		}
	}
	p.reader = reader
}

func (p *payload) Close() {
	if closer, ok := p.reader.(io.Closer); ok {
		closer.Close()
	}
}

type Protocol struct {
}

var _ protocols.Protocol = (*Protocol)(nil)

func (p *Protocol) CreateRequest(req interface{}) protocols.Request {
	r := req.(*http.Request)
	return newRequest(r)
}

func (p *Protocol) CreateResponse(resp interface{}) protocols.Response {
	w := resp.(http.ResponseWriter)
	return newResponse(w)
}

func (p *Protocol) CreateLoadBalancer(lb string, servers []*protocols.Server) (protocols.LoadBalancer, error) {
	return nil, nil
}

func (p *Protocol) CreateServer(uri string) (*protocols.Server, error) {
	return nil, nil
}

func (p *Protocol) CreateTrafficMatcher(spec interface{}) (protocols.TrafficMatcher, error) {
	return nil, nil
}
