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

package mqttproxy

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/cluster"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type mockCluster struct {
	sync.RWMutex
	kv map[string]string
}

var _ cluster.Cluster = (*mockCluster)(nil)

func (m *mockCluster) IsLeader() bool                              { return false }
func (m *mockCluster) Layout() *cluster.Layout                     { return nil }
func (m *mockCluster) GetRaw(key string) (*mvccpb.KeyValue, error) { return nil, nil }
func (m *mockCluster) GetRawPrefix(prefix string) (map[string]*mvccpb.KeyValue, error) {
	return nil, nil
}
func (m *mockCluster) PutUnderLease(key, value string) error                      { return nil }
func (m *mockCluster) PutAndDelete(map[string]*string) error                      { return nil }
func (m *mockCluster) PutAndDeleteUnderLease(map[string]*string) error            { return nil }
func (m *mockCluster) Delete(key string) error                                    { return nil }
func (m *mockCluster) DeletePrefix(prefix string) error                           { return nil }
func (m *mockCluster) STM(apply func(concurrency.STM) error) error                { return nil }
func (m *mockCluster) Watcher() (cluster.Watcher, error)                          { return nil, nil }
func (m *mockCluster) Syncer(pullInterval time.Duration) (*cluster.Syncer, error) { return nil, nil }
func (m *mockCluster) Mutex(name string) (cluster.Mutex, error)                   { return nil, nil }
func (m *mockCluster) CloseServer(wg *sync.WaitGroup)                             {}
func (m *mockCluster) StartServer() (chan struct{}, chan struct{}, error)         { return nil, nil, nil }
func (m *mockCluster) Close(wg *sync.WaitGroup)                                   {}
func (m *mockCluster) PurgeMember(member string) error                            { return nil }

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
	valmap, err := store.getPrefix("prefix")
	if err != nil || !reflect.DeepEqual(valmap, map[string]string{"prefix_1": "1", "prefix_2": "2"}) {
		t.Errorf("get wrong prefix val")
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
	valMap, err := store.getPrefix("key")
	if err != nil || !reflect.DeepEqual(valMap, map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"}) {
		t.Errorf("mock storage get prefix return wrong value %v %v", valMap, map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"})
	}
}

type mockAsyncProducer struct {
	ch chan *sarama.ProducerMessage
}

func (m *mockAsyncProducer) AsyncClose()                               {}
func (m *mockAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (m *mockAsyncProducer) Errors() <-chan *sarama.ProducerError      { return nil }

func (m *mockAsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return m.ch
}
func (m *mockAsyncProducer) Close() error {
	return fmt.Errorf("mock producer close failed")
}

var _ sarama.AsyncProducer = (*mockAsyncProducer)(nil)

func newMockAsyncProducer() sarama.AsyncProducer {
	return &mockAsyncProducer{
		ch: make(chan *sarama.ProducerMessage, 1),
	}
}

func TestKafka(t *testing.T) {
	k := newBackendMQ(&Spec{
		BackendType: kafkaType,
		Kafka: &KafkaSpec{
			Backend: []string{"localhost:1234"},
		},
	})
	if k.(*KafkaMQ) != nil {
		t.Errorf("should return nil for invalid broker address, %v", k)
	}
	k = newBackendMQ(&Spec{
		BackendType: "FakeType",
	})
	if k != nil {
		t.Errorf("should return nil for invalid wrong type")
	}

	mapFunc := func(mqttTopic string) (string, map[string]string, error) {
		levels := strings.Split(mqttTopic, "/")
		m := make(map[string]string)
		for i, l := range levels {
			m[strconv.Itoa(i)] = l
		}
		return mqttTopic, m, nil
	}
	kafka := KafkaMQ{
		producer: newMockAsyncProducer(),
		mapFunc:  mapFunc,
		done:     make(chan struct{}),
	}
	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.TopicName = "a/b/c"
	p.Payload = []byte("abc")

	kafka.publish(p)
	msg := <-kafka.producer.(*mockAsyncProducer).ch
	if msg.Topic != p.TopicName || len(msg.Headers) != 3 {
		t.Errorf("kafka producer produce wrong msg")
	}

	kafka.mapFunc = nil
	kafka.publish(p)
	msg = <-kafka.producer.(*mockAsyncProducer).ch
	if msg.Topic != p.TopicName || len(msg.Headers) != 0 {
		t.Errorf("kafka producer produce wrong msg")
	}
	kafka.close()
}
