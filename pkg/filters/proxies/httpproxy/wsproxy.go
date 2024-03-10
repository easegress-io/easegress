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

package httpproxy

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

// WebSocketProxyKind is the kind of WebSocketProxy.
const WebSocketProxyKind = "WebSocketProxy"

var kindWebSocketProxy = &filters.Kind{
	Name:        WebSocketProxyKind,
	Description: "WebSocketProxy is the proxy for web sockets",
	Results: []string{
		resultInternalError,
		resultClientError,
	},
	DefaultSpec: func() filters.Spec {
		return &WebSocketProxySpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &WebSocketProxy{
			super: spec.Super(),
			spec:  spec.(*WebSocketProxySpec),
		}
	},
}

var _ filters.Filter = (*WebSocketProxy)(nil)

func init() {
	filters.Register(kindWebSocketProxy)
}

type (
	// WebSocketProxy is the filter WebSocketProxy.
	//
	// TODO: it is better to put filters to their own folders,
	// so we need a refactor to extract the WebSocketProxy into
	// its own folder later.
	WebSocketProxy struct {
		super *supervisor.Supervisor
		spec  *WebSocketProxySpec

		mainPool       *WebSocketServerPool
		candidatePools []*WebSocketServerPool
	}

	// WebSocketProxySpec describes the WebSocketProxy.
	WebSocketProxySpec struct {
		filters.BaseSpec `json:",inline"`
		Pools            []*WebSocketServerPoolSpec `json:"pools" jsonschema:"required"`
	}
)

// Validate validates Spec.
func (s *WebSocketProxySpec) Validate() error {
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

// Name returns the name of the WebSocketProxy filter instance.
func (p *WebSocketProxy) Name() string {
	return p.spec.Name()
}

// Kind returns the kind of WebSocketProxy.
func (p *WebSocketProxy) Kind() *filters.Kind {
	return kindWebSocketProxy
}

// Spec returns the spec used by the WebSocketProxy
func (p *WebSocketProxy) Spec() filters.Spec {
	return p.spec
}

// Init initializes WebSocketProxy.
func (p *WebSocketProxy) Init() {
	p.reload()
}

// Inherit inherits previous generation of WebSocketProxy.
func (p *WebSocketProxy) Inherit(previousGeneration filters.Filter) {
	p.reload()
}

func (p *WebSocketProxy) reload() {
	for _, spec := range p.spec.Pools {
		name := ""
		if spec.Filter == nil {
			name = fmt.Sprintf("websocketproxy#%s#main", p.Name())
		} else {
			id := len(p.candidatePools)
			name = fmt.Sprintf("websocketproxy#%s#candidate#%d", p.Name(), id)
		}

		pool := NewWebSocketServerPool(p, spec, name)

		if spec.Filter == nil {
			p.mainPool = pool
		} else {
			p.candidatePools = append(p.candidatePools, pool)
		}
	}
}

// Status returns WebSocketProxy status.
func (p *WebSocketProxy) Status() interface{} {
	s := &Status{
		MainPool: p.mainPool.status(),
	}

	for _, pool := range p.candidatePools {
		s.CandidatePools = append(s.CandidatePools, pool.status())
	}

	return s
}

// Close closes WebSocketProxy.
func (p *WebSocketProxy) Close() {
	p.mainPool.Close()

	for _, v := range p.candidatePools {
		v.Close()
	}
}

// Handle handles HTTPContext.
func (p *WebSocketProxy) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)

	sp := p.mainPool
	for _, v := range p.candidatePools {
		if v.filter.Match(req) {
			sp = v
			break
		}
	}

	return sp.handle(ctx)
}
