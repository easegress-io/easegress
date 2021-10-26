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

type mockMQTTFilter struct {
	mockFilter

	mu      sync.Mutex
	spec    *MockMQTTSpec
	clients map[string]int
}

type MockMQTTSpec struct {
	UserName    string `yaml:"userName" jsonschema:"required"`
	Port        uint16 `yaml:"port" jsonschema:"required"`
	BackendType string `yaml:"backendType" jsonschema:"required"`
}

var _ MQTTFilter = (*mockMQTTFilter)(nil)

func (m *mockMQTTFilter) Kind() string {
	return "MockMQTTFilter"
}

func (m *mockMQTTFilter) DefaultSpec() interface{} {
	return &MockMQTTSpec{}
}

func (m *mockMQTTFilter) Init(filterSpec *FilterSpec) {
	m.spec = filterSpec.FilterSpec().(*MockMQTTSpec)
	m.clients = make(map[string]int)
}

func (m *mockMQTTFilter) HandleMQTT(ctx context.MQTTContext) MQTTResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[ctx.Client().ClientID()]++
	return nil
}

func (m *mockMQTTFilter) clientCount() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ans := make(map[string]int)
	for k, v := range m.clients {
		ans[k] = v
	}
	return ans
}

type mockMQTTClient struct {
	cid string
}

var _ context.MQTTClient = (*mockMQTTClient)(nil)

func (c *mockMQTTClient) ClientID() string {
	return c.cid
}
