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

package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Server is proxy server.
type Server struct {
	URL            string   `yaml:"url" jsonschema:"required,format=url"`
	Tags           []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
	Weight         int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	addrIsHostName bool
}

// String implements the Stringer interface.
func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// checkAddrPattern checks whether the server address is host name or ip:port,
// not all error cases are handled.
func (s *Server) checkAddrPattern() {
	u, err := url.Parse(s.URL)
	if err != nil {
		return
	}
	host := u.Host

	square := strings.LastIndexByte(host, ']')
	colon := strings.LastIndexByte(host, ':')

	// There is a port number, remove it.
	if colon > square {
		host = host[:colon]
	}

	// IPv6
	if square != -1 && host[0] == '[' {
		host = host[1:square]
	}

	s.addrIsHostName = net.ParseIP(host) == nil
}

/*
func (s *Server) SendRequest(req *httpprot.Request) (*httpprot.Response, error) {
	stdr := req.Std()
	statResult := &httpstat.Result{}

	url := s.URL + req.Path()
	if stdr.URL.RawQuery != "" {
		url += "?" + stdr.URL.RawQuery
	}

	ctx := httpstat.WithHTTPStat(stdr.Context(), statResult)
	stdr, err := http.NewRequestWithContext(ctx, req.Method(), url, req.GetPayload())
	if err != nil {
	}

	stdr.Header = req.HTTPHeader()
	if !s.addrIsHostName {
		stdr.Host = req.Host()
	}

	start := fasttime.Now()
	spanName := sp.Spec.SpanName
	if spanName == "" {
		spanName = s.URL
	}

	span := ctx.Span().NewChildWithStart(spanName, start)
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(stdr.Header))

	resp, err := fnSendRequest(stdr, client)
	if err != nil {
		return nil, nil, err
	}
	return resp, span, nil

	return nil, nil
}

*/
