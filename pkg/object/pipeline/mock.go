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

package pipeline

import (
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/supervisor"
)

type mockFilter struct {
}

type mockSpec struct {
}

var _ Filter = (*mockFilter)(nil)

func (f *mockFilter) Kind() string                                              { return "MockFilter" }
func (f *mockFilter) DefaultSpec() interface{}                                  { return &mockSpec{} }
func (f *mockFilter) Description() string                                       { return "mock filter" }
func (f *mockFilter) Results() []string                                         { return nil }
func (f *mockFilter) Init(filterSpec *FilterSpec)                               {}
func (f *mockFilter) Inherit(filterSpec *FilterSpec, previousGeneration Filter) {}
func (f *mockFilter) Status() interface{}                                       { return nil }
func (f *mockFilter) Close()                                                    {}
func (f *mockFilter) APIs() []*APIEntry                                         { return nil }

// MockMQTTFilter is used for test pipeline, which will count the client number of MQTTContext
type MockMQTTFilter struct {
	mockFilter
	mu sync.Mutex

	spec       *MockMQTTSpec
	clients    map[string]int
	disconnect map[string]struct{}
}

// MockMQTTSpec is spec of MockMQTTFilter
type MockMQTTSpec struct {
	UserName               string   `yaml:"userName" jsonschema:"required"`
	Password               string   `yaml:"password" jsonschema:"required"`
	Port                   uint16   `yaml:"port" jsonschema:"required"`
	BackendType            string   `yaml:"backendType" jsonschema:"required"`
	EarlyStop              bool     `yaml:"earlyStop" jsonschema:"omitempty"`
	KeysToStore            []string `yaml:"keysToStore" jsonschema:"omitempty"`
	ConnectKey             string   `yaml:"connectKey" jsonschema:"omitempty"`
	PublishBackendClientID bool     `yaml:"publishBackendClientID" jsonschema:"omitempty"`
}

// MockMQTTStatus is status of MockMQTTFilter
type MockMQTTStatus struct {
	ClientCount      map[string]int
	ClientDisconnect map[string]struct{}
}

var _ MQTTFilter = (*MockMQTTFilter)(nil)

// Kind retrun kind of MockMQTTFilter
func (m *MockMQTTFilter) Kind() string {
	return "MockMQTTFilter"
}

// DefaultSpec retrun default spec of MockMQTTFilter
func (m *MockMQTTFilter) DefaultSpec() interface{} {
	return &MockMQTTSpec{}
}

// Init init MockMQTTFilter
func (m *MockMQTTFilter) Init(filterSpec *FilterSpec) {
	m.spec = filterSpec.FilterSpec().(*MockMQTTSpec)
	m.clients = make(map[string]int)
	m.disconnect = make(map[string]struct{})
}

// HandleMQTT handle MQTTContext
func (m *MockMQTTFilter) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[ctx.Client().ClientID()]++
	if ctx.PacketType() == context.MQTTConnect {
		if ctx.ConnectPacket().Username != m.spec.UserName || string(ctx.ConnectPacket().Password) != m.spec.Password {
			ctx.SetDisconnect()
		}
	}
	if ctx.PacketType() == context.MQTTDisconnect {
		m.disconnect[ctx.Client().ClientID()] = struct{}{}
	}
	for _, k := range m.spec.KeysToStore {
		ctx.Client().Store(k, struct{}{})
	}
	if ctx.PacketType() == context.MQTTConnect {
		ctx.Client().Store(m.spec.ConnectKey, struct{}{})
	}
	if m.spec.PublishBackendClientID {
		backend := ctx.Backend()
		backend.Publish(ctx.Client().ClientID(), nil, nil)
	}
	return nil
}

func (m *MockMQTTFilter) clientCount() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ans := make(map[string]int)
	for k, v := range m.clients {
		ans[k] = v
	}
	return ans
}

func (m *MockMQTTFilter) clientDisconnect() map[string]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	ans := make(map[string]struct{})
	for k, v := range m.disconnect {
		ans[k] = v
	}
	return ans
}

// Status return status of MockMQTTFilter
func (m *MockMQTTFilter) Status() interface{} {
	return MockMQTTStatus{
		ClientCount:      m.clientCount(),
		ClientDisconnect: m.clientDisconnect(),
	}
}

type mockMQTTClient struct {
	cid      string
	userName string
	kvMap    sync.Map
}

var _ context.MQTTClient = (*mockMQTTClient)(nil)

// ClientID return client id of mockMQTTClient
func (c *mockMQTTClient) ClientID() string {
	return c.cid
}

// UserName return username of mockMQTTClient
func (c *mockMQTTClient) UserName() string {
	return c.userName
}

func (c *mockMQTTClient) Load(key interface{}) (interface{}, bool) {
	return c.kvMap.Load(key)
}

func (c *mockMQTTClient) Store(key interface{}, value interface{}) {
	c.kvMap.Store(key, value)
}

func (c *mockMQTTClient) Delete(key interface{}) {
	c.kvMap.Delete(key)
}

// MockFilterSpec help to create FilterSpec for test
func MockFilterSpec(super *supervisor.Supervisor, rawSpec map[string]interface{}, yamlConfig string,
	meta *FilterMetaSpec, filterSpec interface{}) *FilterSpec {
	return &FilterSpec{
		super:      super,
		rawSpec:    rawSpec,
		yamlConfig: yamlConfig,
		meta:       meta,
		filterSpec: filterSpec,
	}
}
