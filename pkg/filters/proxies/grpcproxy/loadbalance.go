/*
 * Copyright (c) 2017, The Easegress Authors
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

package grpcproxy

import (
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
)

const (
	// LoadBalancePolicyForward is the load balance policy of forward.
	LoadBalancePolicyForward = "forward"
)

type forwardLoadBalancer struct {
	servers    sync.Pool
	forwardKey string
}

func newForwardLoadBalancer(spec *LoadBalanceSpec) *forwardLoadBalancer {
	return &forwardLoadBalancer{
		servers: sync.Pool{
			New: func() interface{} {
				return &Server{}
			},
		},
		forwardKey: spec.ForwardKey,
	}
}

// ChooseServer implements the LoadBalancer interface
func (f *forwardLoadBalancer) ChooseServer(req protocols.Request) *Server {
	grpcreq, ok := req.(*grpcprot.Request)
	if !ok {
		panic("not a gRPC request")
	}

	target := grpcreq.RawHeader().GetFirst(f.forwardKey)
	if target == "" {
		logger.Debugf("request %v from %v context no target address %s", grpcreq.FullMethod(), grpcreq.RealIP(), target)
		return nil
	}

	if s, ok := f.servers.Get().(*Server); ok {
		s.URL = target
		return s
	}

	return nil
}

// ReturnServer returns the server to the load balancer.
func (f *forwardLoadBalancer) ReturnServer(s *Server, req protocols.Request, resp protocols.Response) {
	f.servers.Put(s)
}

// Close closes the load balancer.
func (f *forwardLoadBalancer) Close() {
}
