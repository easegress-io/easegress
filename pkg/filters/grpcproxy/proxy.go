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

package grpcprxoy

import (
	"fmt"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/pkg/util/connectionpool"
	grpcpool "github.com/megaease/easegress/pkg/util/connectionpool/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
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
		return &Spec{
			UseConnectionPool: false,
			BorrowTimeout:     "1000ms",
			ConnectTimeout:    "200ms",
			MaxConnsPerHost:   4,
			InitConnNum:       1,
		}
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

		pool connectionpool.Pool
		// locker and conns for not use connection pool
		locker     sync.Mutex
		conns      map[string]*Conn
		closeEvent chan struct{}
	}
	// Conn is wrapper grpc.ClientConn
	Conn struct {
		*grpc.ClientConn
		key     string
		proxy   *Proxy
		isClose bool
	}

	// Spec describes the Proxy.
	Spec struct {
		filters.BaseSpec  `json:",inline"`
		Pools             []*ServerPoolSpec `json:"pools" jsonschema:"required"`
		UseConnectionPool bool              `json:"useConnectionPool" jsonschema:"required"`
		BorrowTimeout     string            `json:"borrowTimeout" jsonschema:"omitempty,format=duration"`
		ConnectTimeout    string            `json:"connectTimeout" jsonschema:"omitempty,format=duration"`
		MaxConnsPerHost   uint16            `json:"maxConnsPerHost" jsonschema:"omitempty,minimum=1"`
		InitConnNum       uint16            `json:"initConnNum" jsonschema:"omitempty,minimum=1"`
	}
)

// monitor 5m timer to remove conn that state is shutdown when
// Proxy Filter Spec.UseConnectionPool is false
func (p *Proxy) monitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.locker.Lock()
			for addr, conn := range p.conns {
				if conn.GetState() == connectivity.Shutdown {
					delete(p.conns, addr)
					conn.isClose = true
				}
			}
			p.locker.Unlock()
		case <-p.closeEvent:
			return
		}
	}
}

// Close to close Conn
func (c *Conn) Close() {
	if c.isClose {
		return
	}
	c.proxy.locker.Lock()
	defer c.proxy.locker.Unlock()
	defer func() { c.isClose = true }()
	if c.proxy.conns[c.key] != c {
		return
	}
	delete(c.proxy.conns, c.key)
}

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

	var (
		borrowTimeout  time.Duration
		err            error
		connectTimeout time.Duration
	)

	if s.BorrowTimeout != "" {
		if borrowTimeout, err = time.ParseDuration(s.BorrowTimeout); err != nil || borrowTimeout == 0 {
			return fmt.Errorf("grpc client borrow timeout %s invalid", s.BorrowTimeout)
		}
	}
	if s.ConnectTimeout != "" {
		if connectTimeout, err = time.ParseDuration(s.ConnectTimeout); err != nil || connectTimeout == 0 {
			return fmt.Errorf("grpc client wait connection ready timeout %s invalid", s.ConnectTimeout)
		}

	}
	if borrowTimeout <= connectTimeout {
		return fmt.Errorf("grpc client borrow connection timeout %v should greater than connect timeout %v", borrowTimeout, connectTimeout)
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

	p.closeEvent = make(chan struct{}, 1)

	if !p.spec.UseConnectionPool {
		p.conns = make(map[string]*Conn)
		go p.monitor()
	} else {
		// already valid in Spec.Validate()
		borrowTimeout, _ := time.ParseDuration(p.spec.BorrowTimeout)
		connectTimeout, _ := time.ParseDuration(p.spec.ConnectTimeout)
		poolSpec := &grpcpool.Spec{
			InitConnNum:     p.spec.InitConnNum,
			BorrowTimeout:   borrowTimeout,
			ConnectTimeout:  connectTimeout,
			MaxConnsPerHost: p.spec.MaxConnsPerHost,
			DialOptions:     []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithCodec(grpcpool.GetCodecInstance())},
		}

		p.pool = grpcpool.NewPool(poolSpec)
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

	// help gc
	if !p.spec.UseConnectionPool {
		p.conns = nil
	} else {
		p.pool.Close()
		p.pool = nil
	}
	p.closeEvent <- struct{}{}
	close(p.closeEvent)
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
