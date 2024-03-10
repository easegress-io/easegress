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
	stdctx "context"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func newTestProxy(yamlSpec string, assert *assert.Assertions) *Proxy {
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlSpec), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assert.NoError(err)

	proxy := kind.CreateInstance(spec).(*Proxy)
	proxy.Init()

	assert.Nil(proxy.Status())
	assert.Equal(kind, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func TestSpecValidate(t *testing.T) {
	spec := &Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: "grpcforwardproxy",
				Kind: "GRPCProxy",
			},
		},
		ConnectTimeout:      "3q",
		BorrowTimeout:       "4q",
		Timeout:             "5q",
		MaxIdleConnsPerHost: -1,
		Pools: []*ServerPoolSpec{
			{
				BaseServerPoolSpec: proxies.ServerPoolBaseSpec{
					ServiceName: "easegress",
					LoadBalance: &LoadBalanceSpec{
						Policy: "forward",
					},
				},
			},
			{},
		},
	}
	assert.Error(t, spec.Validate())

	spec.Pools[1] = &ServerPoolSpec{
		BaseServerPoolSpec: proxies.ServerPoolBaseSpec{
			ServiceName: "easegress",
			LoadBalance: &LoadBalanceSpec{
				Policy: "forward",
			},
		},
	}
	assert.Error(t, spec.Validate())

	spec.Pools[1].Filter = &RequestMatcherSpec{}
	assert.Error(t, spec.Validate())

	spec.ConnectTimeout = "3s"
	assert.Error(t, spec.Validate())

	spec.BorrowTimeout = "4s"
	assert.Error(t, spec.Validate())

	spec.Timeout = "5s"
	assert.Error(t, spec.Validate())

	spec.MaxIdleConnsPerHost = 10
	assert.NoError(t, spec.Validate())

	spec.ConnectTimeout = ""
	spec.BorrowTimeout = ""
	spec.Timeout = ""
	assert.NoError(t, spec.Validate())
}

func TestReload(t *testing.T) {
	assertions := assert.New(t)
	s := `
kind: GRPCProxy
pools:
 - loadBalance:
     policy: forward
   serviceName: easegress
maxIdleConnsPerHost: 10
initConnsPerHost: 1
connectTimeout: 1s
borrowTimeout: 1s
name: grpcforwardproxy
`
	p := newTestProxy(s, assertions)
	oldPool := p.connectionPool

	s = `
kind: GRPCProxy
pools:
 - loadBalance:
     policy: forward
   serviceName: easegress
 - loadBalance:
     policy: forward
   serviceName: easegress
   filter:
     policy: random
     permil: 50
maxIdleConnsPerHost: 2
initConnsPerHost: 2
connectTimeout: 100ms
borrowTimeout: 100ms
name: grpcforwardproxy
`
	rawSpec := make(map[string]interface{})
	_ = yaml.Unmarshal([]byte(s), &rawSpec)
	_ = codectool.Unmarshal([]byte(s), &rawSpec)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assertions.Nil(err)
	p.spec = spec.(*Spec)
	p.reload()
	assertions.True(oldPool == p.connectionPool)

	p1 := &Proxy{spec: p.spec}
	p1.Inherit(p)
	p.Close()
	p1.Close()
}

func TestHandle(t *testing.T) {
	assertions := assert.New(t)

	s := `
kind: GRPCProxy
pools:
 - loadBalance:
     policy: forward
   serviceName: easegress
   circuitBreakerPolicy: circuitbreak
 - loadBalance:
     policy: forward
   serviceName: easegress
   filter:
     policy: random
     permil: 50
maxIdleConnsPerHost: 2
initConnsPerHost: 2
connectTimeout: 100ms
borrowTimeout: 100ms
name: grpcforwardproxy
`
	p := newTestProxy(s, assertions)
	assert.Panics(t, func() {
		p.InjectResiliencePolicy(map[string]resilience.Policy{
			"circuitbreak": resilience.RetryKind.DefaultPolicy(),
		})
	})
	p.InjectResiliencePolicy(map[string]resilience.Policy{
		"circuitbreak": resilience.CircuitBreakerKind.DefaultPolicy(),
	})
	ctx := context.New(nil)

	stream := grpcprot.NewFakeServerStream(stdctx.Background())
	req := grpcprot.NewRequestWithServerStream(stream)
	ctx.SetInputRequest(req)

	result := p.Handle(ctx)
	assert.NotEmpty(t, result)

	assert.Nil(t, p.Status())
	p.Close()
}
