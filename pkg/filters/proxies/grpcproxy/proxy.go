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

package grpcproxy

import (
	"fmt"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/filters/proxies"
	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Kind is the kind of Proxy.
	Kind = "GRPCProxy"

	resultInternalError = "internalError"
	resultClientError   = "clientError"
	resultServerError   = "serverError"

	// result for resilience
	resultShortCircuited = "shortCircuited"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "GRPCProxy sets the proxy of grpc servers",
	Results: []string{
		resultInternalError,
		resultClientError,
		resultServerError,
		resultShortCircuited,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Proxy{
			super: spec.Super(),
			spec:  spec.(*Spec),
		}
	},
}

var _ filters.Filter = (*Proxy)(nil)
var _ filters.Resiliencer = (*Proxy)(nil)

func init() {
	filters.Register(kind)
}

type (
	// Proxy is the filter Proxy.
	Proxy struct {
		super *supervisor.Supervisor
		spec  *Spec

		mainPool       *ServerPool
		candidatePools []*ServerPool
	}

	// Spec describes the Proxy.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		Pools            []*ServerPoolSpec `json:"pools" jsonschema:"required"`
	}

	// Server is the backend server.
	Server = proxies.Server
)

// Validate validates Spec.
func (s *Spec) Validate() error {
	numMainPool := 0
	for i, pool := range s.Pools {
		if pool.Filter == nil {
			numMainPool++
		}
		if err := pool.Validate(); err != nil {
			return fmt.Errorf("pool %d: %v", i, err)
		}
	}

	if numMainPool != 1 {
		return fmt.Errorf("one and only one mainPool is required")
	}
	return nil
}

// Name returns the name of the Proxy filter instance.
func (p *Proxy) Name() string {
	return p.spec.Name()
}

// Kind returns the kind of Proxy.
func (p *Proxy) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Proxy
func (p *Proxy) Spec() filters.Spec {
	return p.spec
}

// Init initializes Proxy.
func (p *Proxy) Init() {
	p.reload()
}

// Inherit inherits previous generation of Proxy.
func (p *Proxy) Inherit(previousGeneration filters.Filter) {
	p.reload()
}

func (p *Proxy) reload() {
	for _, spec := range p.spec.Pools {
		name := ""
		if spec.Filter == nil {
			name = fmt.Sprintf("proxy#%s#main", p.Name())
		} else {
			id := len(p.candidatePools)
			name = fmt.Sprintf("proxy#%s#candidate#%d", p.Name(), id)
		}

		pool := NewServerPool(p, spec, name)

		if spec.Filter == nil {
			p.mainPool = pool
		} else {
			p.candidatePools = append(p.candidatePools, pool)
		}
	}
}

// Status returns Proxy status.
func (p *Proxy) Status() interface{} {
	return nil
}

// Close closes Proxy.
func (p *Proxy) Close() {
	p.mainPool.close()

	for _, v := range p.candidatePools {
		v.close()
	}
}

// Handle handles GRPCContext.
func (p *Proxy) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*grpcprot.Request)
	sp := p.mainPool
	for _, v := range p.candidatePools {
		if v.filter.Match(req) {
			sp = v
			break
		}
	}

	return sp.handle(ctx)
}

// InjectResiliencePolicy injects resilience policies to the proxy.
func (p *Proxy) InjectResiliencePolicy(policies map[string]resilience.Policy) {
	p.mainPool.InjectResiliencePolicy(policies)

	for _, sp := range p.candidatePools {
		sp.InjectResiliencePolicy(policies)
	}
}
