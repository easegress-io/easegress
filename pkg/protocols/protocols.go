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

package protocols

var registry = map[string]Protocol{}

func Register(name string, p Protocol) {
	registry[name] = p
}

// Get returns protocol by name.
func Get(name string) Protocol {
	return registry[name]
}

// Request is the protocol independent interface of a request.
type Request interface {
	Finish()
	Clone() Request
}

// Response is the protocol independent interface of a response.
type Response interface {
	Finish()
	Clone() Response
}

type server interface {
	SendRequest(req Request) (Response, error)
}

type Server struct {
	Weight int
	server server
}

// LoadBalancer is the protocol independent interface of a load balancer.
type LoadBalancer interface {
	ChooseServer(req Request) *Server
}

// TrafficMatcher is the protocol independent interface to match traffics.
type TrafficMatcher interface {
	Match(req Request) bool
}

// Protocol is the interface of a protocol.
type Protocol interface {
	CreateRequest() Request
	CreateResponse() Response
	CreateLoadBalancer(lb string, servers []*Server) (LoadBalancer, error)
	CreateServer(uri string) (*Server, error)
	CreateTrafficMatcher(spec interface{}) (TrafficMatcher, error)
}
