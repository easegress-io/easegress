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
	return ctx
}

func TestKafka(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{
		Backend: []string{"localhost:1234"},
	}
	filterSpec := defaultFilterSpec(spec)
	k := kind.CreateInstance(filterSpec)
	assert.Equal(&Spec{}, kind.DefaultSpec())
	assert.Panics(func() { k.Init() }, "kafka should panic for invalid backend")
	assert.Equal(spec.BaseSpec.MetaSpec.Name, k.Name())
	assert.Equal(kind, k.Kind())
	assert.Equal(filterSpec, k.Spec())
	assert.Nil(k.Status())

	kafka := Kafka{
		producer: newMockAsyncProducer(),
		done:     make(chan struct{}),
	}

	mqttCtx := newContext("test", "user123", "a/b/c", []byte("text"))
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
	assert.Panics(func() { newK.Inherit(k) })
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

	newAsyncProducer = func(addrs []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
		return newMockAsyncProducer(), nil
	}
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
