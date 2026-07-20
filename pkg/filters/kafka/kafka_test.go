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

package kafka

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"

	"github.com/Shopify/sarama"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

type mockAsyncProducer struct {
	ch      chan *sarama.ProducerMessage
	errorCh chan *sarama.ProducerError
	closed  int32
}

func (m *mockAsyncProducer) IsTransactional() bool {
	return false
}

func (m *mockAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	return 0
}

func (m *mockAsyncProducer) BeginTxn() error {
	return nil
}

func (m *mockAsyncProducer) CommitTxn() error {
	return nil
}

func (m *mockAsyncProducer) AbortTxn() error {
	return nil
}

func (m *mockAsyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupID string) error {
	return nil
}

func (m *mockAsyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupID string, metadata *string) error {
	return nil
}

func (m *mockAsyncProducer) AsyncClose()                               {}
func (m *mockAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (m *mockAsyncProducer) Errors() <-chan *sarama.ProducerError      { return m.errorCh }

func (m *mockAsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return m.ch
}
func (m *mockAsyncProducer) Close() error {
	atomic.StoreInt32(&m.closed, 1)
	return fmt.Errorf("mock producer close failed")
}

var _ sarama.AsyncProducer = (*mockAsyncProducer)(nil)

func newMockAsyncProducer() sarama.AsyncProducer {
	return &mockAsyncProducer{
		ch:      make(chan *sarama.ProducerMessage, 100),
		errorCh: make(chan *sarama.ProducerError),
	}
}

func defaultFilterSpec(spec *Spec) filters.Spec {
	spec.BaseSpec.MetaSpec.Kind = Kind
	spec.BaseSpec.MetaSpec.Name = "kafka-demo"
	return spec
}

func newContext(cid string, username string, topic string, payload []byte) *context.Context {
	ctx := context.New(nil)

	client := &mqttprot.MockClient{
		MockClientID: cid,
		MockUserName: username,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	packet.Payload = payload
	req := mqttprot.NewRequest(packet, client)

	ctx.SetInputRequest(req)
	ctx.SetResponse(context.DefaultNamespace, mqttprot.NewResponse())
	return ctx
}

func setTestProducerRetryBackoff(t *testing.T, backoff time.Duration) {
	t.Helper()
	oldRetryBackoff := producerRetryBackoff
	oldMaxRetryBackoff := producerMaxRetryBackoff
	producerRetryBackoff = backoff
	producerMaxRetryBackoff = backoff
	t.Cleanup(func() {
		producerRetryBackoff = oldRetryBackoff
		producerMaxRetryBackoff = oldMaxRetryBackoff
	})
}

func setTestProducerFactory(t *testing.T, factory func([]string, *sarama.Config) (sarama.AsyncProducer, error)) {
	t.Helper()
	oldFactory := newAsyncProducer
	newAsyncProducer = factory
	t.Cleanup(func() {
		newAsyncProducer = oldFactory
	})
}

func TestKafka(t *testing.T) {
	assert := assert.New(t)
	setTestProducerRetryBackoff(t, 10*time.Millisecond)
	setTestProducerFactory(t, func(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
		return nil, fmt.Errorf("mock producer unavailable")
	})

	spec := &Spec{
		Backend: []string{"localhost:1234"},
	}
	filterSpec := defaultFilterSpec(spec)
	k := kind.CreateInstance(filterSpec)
	assert.Equal(&Spec{}, kind.DefaultSpec())
	assert.NotPanics(func() { k.Init() }, "kafka should retry for invalid backend")
	defer k.Close()
	assert.Equal(spec.BaseSpec.MetaSpec.Name, k.Name())
	assert.Equal(kind, k.Kind())
	assert.Equal(filterSpec, k.Spec())
	status := k.Status().(*Status)
	assert.False(status.Ready)
	assert.Contains(status.Health, "mock producer unavailable")

	mqttCtx := newContext("test", "user123", "a/b/c", []byte("text"))
	assert.Equal(resultProducerUnavailable, k.Handle(mqttCtx))
	assert.True(mqttCtx.GetOutputResponse().(*mqttprot.Response).Drop())

	kafka := Kafka{
		producer: newMockAsyncProducer(),
		done:     make(chan struct{}),
	}

	mqttCtx = newContext("test", "user123", "a/b/c", []byte("text"))
	kafka.Handle(mqttCtx)
	msg := <-kafka.producer.(*mockAsyncProducer).ch

	req := mqttCtx.GetInputRequest().(*mqttprot.Request)
	assert.Equal(msg.Topic, req.PublishPacket().TopicName)

	assert.Equal(3, len(msg.Headers))
	headerMap := map[string]string{
		"clientID":  "test",
		"mqttTopic": "a/b/c",
		"username":  "user123",
	}
	for _, h := range msg.Headers {
		assert.Equal(headerMap[string(h.Key)], string(h.Value))
	}

	value, err := msg.Value.Encode()
	assert.Nil(err)
	assert.Equal("text", string(value))

	newK := kind.CreateInstance(filterSpec)
	assert.NotPanics(func() { newK.Inherit(k) })
	newK.Close()
}

func TestKafkaWithKVMap(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{
		Backend: []string{"localhost:1234"},
		KVMap: &KVMap{
			TopicKey:  "topic",
			HeaderKey: "headers",
		},
	}

	kafka := Kafka{
		spec:     spec,
		producer: newMockAsyncProducer(),
		done:     make(chan struct{}),
	}
	kafka.setKV()
	defer kafka.Close()

	mqttCtx := newContext("test", "user123", "a/b/c", []byte("text"))
	mqttCtx.SetData("topic", "123")
	mqttCtx.SetData("headers", map[string]string{"1": "a"})

	kafka.Handle(mqttCtx)
	msg := <-kafka.producer.(*mockAsyncProducer).ch
	assert.Equal("123", msg.Topic)
	assert.Equal(4, len(msg.Headers))
	value, err := msg.Value.Encode()
	assert.Nil(err)
	assert.Equal("text", string(value))
}

func TestKafka2(t *testing.T) {
	assert := assert.New(t)

	setTestProducerFactory(t, func(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
		return newMockAsyncProducer(), nil
	})
	spec := &Spec{
		Backend: []string{"localhost:1234"},
		KVMap: &KVMap{
			TopicKey:  "topic",
			HeaderKey: "headers",
		},
	}

	kafka := Kafka{
		spec: spec,
	}
	kafka.Init()
	p := kafka.producer.(*mockAsyncProducer)
	p.errorCh <- &sarama.ProducerError{}

	for i := 0; i < 100; i++ {
		status := kafka.Status().(*Status)
		if status.Health == "sarama producer failed" {
			assert.True(status.Ready)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := kafka.Status().(*Status)
	assert.True(status.Ready)
	assert.Equal("sarama producer failed", status.Health)

	kafka.Close()
	for i := 0; i < 10; i++ {
		closed := atomic.LoadInt32(&p.closed)
		if closed == 1 {
			break
		}
		time.Sleep(time.Second)
	}
	assert.Equal(int32(1), atomic.LoadInt32(&p.closed))
}

func TestKafkaReconnectsAfterInitialProducerFailure(t *testing.T) {
	assert := assert.New(t)
	setTestProducerRetryBackoff(t, 10*time.Millisecond)

	var attempts int32
	mockProducer := newMockAsyncProducer()
	setTestProducerFactory(t, func(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			return nil, fmt.Errorf("mock producer unavailable")
		}
		return mockProducer, nil
	})

	kafka := Kafka{
		spec: &Spec{
			Backend: []string{"localhost:1234"},
		},
	}
	kafka.Init()
	defer kafka.Close()

	mqttCtx := newContext("test", "user123", "a/b/c", []byte("text"))
	assert.Equal(resultProducerUnavailable, kafka.Handle(mqttCtx))
	assert.True(mqttCtx.GetOutputResponse().(*mqttprot.Response).Drop())

	for i := 0; i < 100; i++ {
		if kafka.getProducer() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	status := kafka.Status().(*Status)
	assert.True(status.Ready)
	assert.Equal("ready", status.Health)

	mqttCtx = newContext("test", "user123", "a/b/c", []byte("text"))
	assert.Equal("", kafka.Handle(mqttCtx))
	msg := <-mockProducer.(*mockAsyncProducer).ch
	assert.Equal("a/b/c", msg.Topic)
}

func TestKafkaCloseBeforeReconnect(t *testing.T) {
	assert := assert.New(t)
	setTestProducerRetryBackoff(t, 100*time.Millisecond)

	attempts := make(chan struct{}, 10)
	setTestProducerFactory(t, func(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
		attempts <- struct{}{}
		return nil, fmt.Errorf("mock producer unavailable")
	})

	kafka := Kafka{
		spec: &Spec{
			Backend: []string{"localhost:1234"},
		},
	}
	kafka.Init()
	<-attempts
	kafka.Close()

	select {
	case <-attempts:
		assert.Fail("producer reconnect should stop after Close")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestKafkaSpecAllowsEmptyMQTTMap(t *testing.T) {
	spec := &Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: "kafka-demo",
				Kind: Kind,
			},
		},
		Backend: []string{"localhost:1234"},
		Topic: &Topic{
			Default: "kafka-topic",
		},
	}

	_, err := filters.NewSpec(nil, "pipeline-demo", spec)
	assert.Nil(t, err)
}
