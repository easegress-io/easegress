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

package proxies

import (
	"testing"

	"github.com/megaease/easegress/v2/pkg/object/serviceregistry"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestServerPoolSpecValidate(t *testing.T) {
	assert := assert.New(t)

	spec := &ServerPoolBaseSpec{}
	assert.Error(spec.Validate())

	spec.Servers = []*Server{
		{}, {},
	}

	assert.NoError(spec.Validate())

	spec.Servers[0].Weight = 1
	assert.Error(spec.Validate())

	spec.Servers[1].Weight = 1
	spec.ServiceName = "test_service"
	spec.LoadBalance = &LoadBalanceSpec{
		HealthCheck: &HealthCheckSpec{},
	}
	assert.Error(spec.Validate())
}

type MockServerPoolImpl struct {
}

func (m *MockServerPoolImpl) CreateLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	lb := NewGeneralLoadBalancer(spec, servers)
	lb.Init(NewHTTPSessionSticker, nil, nil)
	return lb
}

func TestUseService(t *testing.T) {
	assert := assert.New(t)

	spec := &ServerPoolBaseSpec{
		ServerTags:  []string{"a2"},
		LoadBalance: &LoadBalanceSpec{},
		Servers: []*Server{
			{
				URL:  "http://192.168.1.1:80",
				Tags: []string{"a2"},
			},
		},
	}

	sp := &ServerPoolBase{
		spImpl: &MockServerPoolImpl{},
	}

	sp.useService(spec, map[string]*serviceregistry.ServiceInstanceSpec{
		"2": {
			Address: "192.168.1.2",
			Tags:    []string{"a2"},
			Port:    80,
		},
		"3": {
			Address: "192.168.1.3",
			Tags:    []string{"a3"},
			Port:    80,
		},
	})
	svr := sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.2:80", svr.URL)
	svr = sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.2:80", svr.URL)

	spec.LoadBalance = nil
	sp.useService(spec, map[string]*serviceregistry.ServiceInstanceSpec{})
	svr = sp.LoadBalancer().ChooseServer(nil)
	assert.Equal("http://192.168.1.1:80", svr.URL)
}

func TestServerPoolInit(t *testing.T) {
	spec := &ServerPoolBaseSpec{
		ServerTags:  []string{"a2"},
		LoadBalance: &LoadBalanceSpec{},
		Servers: []*Server{
			{
				URL:  "http://192.168.1.1:80",
				Tags: []string{"a2"},
			},
		},
	}

	sp := &ServerPoolBase{}
	assert.Nil(t, sp.LoadBalancer())
	sp.Init(&MockServerPoolImpl{}, supervisor.NewDefaultMock(), "test", spec)
	assert.NotNil(t, sp.Done())
	assert.NotNil(t, sp.LoadBalancer())
	sp.Close()
}
