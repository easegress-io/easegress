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

package http

import (
	"net/http"

	"github.com/megaease/easegress/pkg/protocols"
)

func init() {
	protocols.Register("http", &Protocol{})
}

type Request struct {
	http.Request
}

func (r *Request) Clone() protocols.Request {
	n := *r
	return &n
}

func (r *Request) Finish() {
}

type Response struct {
}

func (r *Response) Clone() protocols.Response {
	n := *r
	return &n
}

func (r *Response) Finish() {
}

type Protocol struct {
	http.ResponseWriter
}

func (p *Protocol) CreateRequest() protocols.Request {
	return &Request{}
}

func (p *Protocol) CreateResponse() protocols.Response {
	return &Response{}
}

func (p *Protocol) CreateLoadBalancer(lb string, servers []*Server) (protocols.LoadBalancer, error) {
	return nil, nil
}

func (p *Protocol) CreateServer(uri string) (*Server, error) {
	return nil, nil
}

func (p *Protocol) CreateTrafficMatcher(spec interface{}) (protocols.TrafficMatcher, error) {
	return nil, nil
}
