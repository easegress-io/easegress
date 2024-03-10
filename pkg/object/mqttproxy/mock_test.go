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

package mqttproxy

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type mockCluster struct {
	sync.RWMutex
	kv    map[string]string
	delCh chan map[string]*string
}

var _ cluster.Cluster = (*mockCluster)(nil)

func (m *mockCluster) IsLeader() bool                              { return false }
func (m *mockCluster) Layout() *cluster.Layout                     { return nil }
func (m *mockCluster) GetRaw(key string) (*mvccpb.KeyValue, error) { return nil, nil }
func (m *mockCluster) GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error) {
	return nil, nil
}
func (m *mockCluster) PutUnderLease(key, value string) error                          { return nil }
func (m *mockCluster) PutUnderTimeout(key, value string, timeout time.Duration) error { return nil }
func (m *mockCluster) PutAndDelete(map[string]*string) error                          { return nil }
func (m *mockCluster) PutAndDeleteUnderLease(map[string]*string) error                { return nil }
func (m *mockCluster) DeletePrefix(prefix string) error                               { return nil }
func (m *mockCluster) STM(apply func(concurrency.STM) error) error                    { return nil }
func (m *mockCluster) Syncer(pullInterval time.Duration) (cluster.Syncer, error)      { return nil, nil }
func (m *mockCluster) Mutex(name string) (cluster.Mutex, error)                       { return nil, nil }
func (m *mockCluster) CloseServer(wg *sync.WaitGroup)                                 {}
func (m *mockCluster) StartServer() (chan struct{}, chan struct{}, error)             { return nil, nil, nil }
func (m *mockCluster) Close(wg *sync.WaitGroup)                                       {}
func (m *mockCluster) PurgeMember(member string) error                                { return nil }

func (m *mockCluster) Watcher() (cluster.Watcher, error) {
	m.Lock()
	defer m.Unlock()
	if m.delCh == nil {
		m.delCh = make(chan map[string]*string, 100)
	}
	return &mockWatcher{delCh: m.delCh}, nil
}

func (m *mockCluster) Delete(key string) error {
	m.Lock()
	defer m.Unlock()
	if v, ok := m.kv[key]; ok {
		if m.delCh != nil {
			kv := map[string]*string{key: &v}
			m.delCh <- kv
		}
	}
	delete(m.kv, key)
	return nil
}

type mockWatcher struct {
	delCh chan map[string]*string
}

var _ cluster.Watcher = (*mockWatcher)(nil)

func (w *mockWatcher) Watch(key string) (<-chan *string, error)                     { return nil, nil }
func (w *mockWatcher) WatchPrefix(prefix string) (<-chan map[string]*string, error) { return nil, nil }
func (w *mockWatcher) WatchRaw(key string) (<-chan *clientv3.Event, error)          { return nil, nil }
func (w *mockWatcher) WatchRawPrefix(prefix string) (<-chan map[string]*clientv3.Event, error) {
	return nil, nil
}

func (w *mockWatcher) WatchWithOp(key string, ops ...cluster.ClientOp) (<-chan map[string]*string, error) {
	return w.delCh, nil
}
func (w *mockWatcher) Close() {}

func (m *mockCluster) Get(key string) (*string, error) {
	m.RLock()
	defer m.RUnlock()
	if val, ok := m.kv[key]; ok {
		return &val, nil
	}
	return nil, fmt.Errorf("not find key")
}

func (m *mockCluster) GetPrefix(prefix string) (map[string]string, error) {
	m.RLock()
	out := make(map[string]string)
	for k, v := range m.kv {
		if strings.Contains(k, prefix) {
			out[k] = v
		}
	}
	m.RUnlock()
	return out, nil
}

func (m *mockCluster) GetWithOp(key string, op ...cluster.ClientOp) (map[string]string, error) {
	prefix := false
	for _, o := range op {
		if o == cluster.OpPrefix {
			prefix = true
		}
	}
	ans := make(map[string]string)
	if prefix {
		kvs, _ := m.GetPrefix(key)
		for k, v := range kvs {
			ans[k] = v
		}
	} else {
		value, _ := m.Get(key)
		ans[key] = *value
	}
	return ans, nil
}

func (m *mockCluster) Put(key, value string) error {
	m.Lock()
	m.kv[key] = value
	m.Unlock()
	return nil
}

func newMockCluster() cluster.Cluster {
	return &mockCluster{kv: make(map[string]string)}
}

func TestStorage(t *testing.T) {
	cls := newMockCluster()
	store := newStorage(cls)
	store.put("prefix_1", "1")
	store.put("prefix_2", "2")
	val, err := store.get("prefix_1")
	if err != nil || *val != "1" {
		t.Errorf("get wrong val")
	}
	valmap, err := store.getPrefix("prefix", false)
	if err != nil || !reflect.DeepEqual(valmap, map[string]string{"prefix_1": "1", "prefix_2": "2"}) {
		t.Errorf("get wrong prefix val")
	}
	ch, _, err := store.watch("prefix")
	if err != nil {
		t.Errorf("create watch delete failed %v", err)
	}
	store.delete("prefix_1")
	delKv := <-ch
	v, ok := delKv["prefix_1"]
	if !ok {
		t.Errorf("watch delete failed")
	} else {
		if *v != "1" {
			t.Errorf("get wrong watch result, expected %v, got %v", "1", *v)
		}
	}
}

func TestMockStorage(t *testing.T) {
	store := newStorage(nil)
	store.put("key1", "val1")
	store.put("key2", "val2")
	store.put("key3", "val3")
	val, err := store.get("key1")
	if err != nil || *val != "val1" {
		t.Errorf("mock storage get return wrong value")
	}
	valMap, err := store.getPrefix("key", false)
	if err != nil || !reflect.DeepEqual(valMap, map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"}) {
		t.Errorf("mock storage get prefix return wrong value %v %v", valMap, map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"})
	}
}

type MockKafka struct {
	ch   chan *packets.PublishPacket
	spec *MockKafkaSpec
}

type MockKafkaSpec struct {
	filters.BaseSpec `json:",inline"`
}

var mockKafkaKind = &filters.Kind{
	Name:        "MockKafka",
	Description: "MockKafka filter is used for testing MQTTProxy",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &MockKafkaSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &MockKafka{
			spec: spec.(*MockKafkaSpec),
		}
	},
}

var _ filters.Filter = (*MockKafka)(nil)

func (k *MockKafka) Name() string                    { return k.spec.Name() }
func (k *MockKafka) Kind() *filters.Kind             { return mockKafkaKind }
func (k *MockKafka) Spec() filters.Spec              { return nil }
func (k *MockKafka) Status() interface{}             { return nil }
func (k *MockKafka) Inherit(previous filters.Filter) {}
func (k *MockKafka) Close()                          {}

func (k *MockKafka) Init() {
	k.ch = make(chan *packets.PublishPacket, 100)
}

func (k *MockKafka) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*mqttprot.Request)
	if req.PacketType() != mqttprot.PublishType {
		panic(fmt.Errorf("mock kafka for test should only receive publish packet, but received %v", req.PacketType()))
	}
	k.ch <- req.PublishPacket()
	return ""
}

func (k *MockKafka) get() *packets.PublishPacket {
	p := <-k.ch
	return p
}

var mockMQTTFilterKind = &filters.Kind{
	Name:        "MockMQTTFilter",
	Description: "MockFilter is used for testing MQTTProxy",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &MockMQTTSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &MockMQTTFilter{spec: spec.(*MockMQTTSpec)}
	},
}

var _ filters.Filter = (*MockMQTTFilter)(nil)

// MockMQTTFilter is used for test pipeline, which will count the client number of MQTTContext
type MockMQTTFilter struct {
	mu sync.Mutex

	spec        *MockMQTTSpec
	clients     map[string]int
	disconnect  map[string]struct{}
	subscribe   map[string][]string
	unsubscribe map[string][]string
}

// MockMQTTSpec is spec of MockMQTTFilter
type MockMQTTSpec struct {
	filters.BaseSpec `json:",inline"`
	UserName         string   `json:"userName" jsonschema:"required"`
	Password         string   `json:"password" jsonschema:"required"`
	Port             uint16   `json:"port" jsonschema:"required"`
	BackendType      string   `json:"backendType" jsonschema:"required"`
	EarlyStop        bool     `json:"earlyStop,omitempty"`
	KeysToStore      []string `json:"keysToStore,omitempty"`
	ConnectKey       string   `json:"connectKey,omitempty"`
}

// MockMQTTStatus is status of MockMQTTFilter
type MockMQTTStatus struct {
	ClientCount      map[string]int
	ClientDisconnect map[string]struct{}
	Subscribe        map[string][]string
	Unsubscribe      map[string][]string
}

var _ filters.Filter = (*MockMQTTFilter)(nil)

// Kind retrun kind of MockMQTTFilter
func (m *MockMQTTFilter) Kind() *filters.Kind {
	return mockMQTTFilterKind
}

// Init init MockMQTTFilter
func (m *MockMQTTFilter) Init() {
	m.clients = make(map[string]int)
	m.disconnect = make(map[string]struct{})
	m.subscribe = make(map[string][]string)
	m.unsubscribe = make(map[string][]string)
}

func (m *MockMQTTFilter) Name() string {
	return m.spec.Name()
}

func (m *MockMQTTFilter) Inherit(previous filters.Filter) {
	m.Init()
}

// HandleMQTT handle MQTTContext
func (m *MockMQTTFilter) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*mqttprot.Request)
	resp := ctx.GetOutputResponse().(*mqttprot.Response)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[req.Client().ClientID()]++

	switch req.PacketType() {
	case mqttprot.ConnectType:
		req.Client().Store(m.spec.ConnectKey, struct{}{})
		if req.ConnectPacket().Username != m.spec.UserName || string(req.ConnectPacket().Password) != m.spec.Password {
			resp.SetDisconnect()
		}
	case mqttprot.DisconnectType:
		m.disconnect[req.Client().ClientID()] = struct{}{}
	case mqttprot.SubscribeType:
		m.subscribe[req.Client().ClientID()] = req.SubscribePacket().Topics
	case mqttprot.UnsubscribeType:
		m.unsubscribe[req.Client().ClientID()] = req.UnsubscribePacket().Topics
	}

	for _, k := range m.spec.KeysToStore {
		req.Client().Store(k, struct{}{})
	}
	return ""
}

// Status return status of MockMQTTFilter
func (m *MockMQTTFilter) Status() interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientCount := make(map[string]int)
	for k, v := range m.clients {
		clientCount[k] = v
	}
	disconnect := make(map[string]struct{})
	for k := range m.disconnect {
		disconnect[k] = struct{}{}
	}
	subscribe := make(map[string][]string)
	for k, v := range m.subscribe {
		vv := make([]string, len(v))
		copy(vv, v)
		subscribe[k] = v
	}
	unsubscribe := make(map[string][]string)
	for k, v := range m.unsubscribe {
		vv := make([]string, len(v))
		copy(vv, v)
		unsubscribe[k] = vv
	}
	return MockMQTTStatus{
		ClientCount:      clientCount,
		ClientDisconnect: disconnect,
		Subscribe:        subscribe,
		Unsubscribe:      unsubscribe,
	}
}

func (m *MockMQTTFilter) Spec() filters.Spec {
	return &MockMQTTSpec{}
}

func (m *MockMQTTFilter) Close() {}
