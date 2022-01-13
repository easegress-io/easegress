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

package kafka

import (
	stdcontext "context"
	"fmt"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"

	"github.com/Shopify/sarama"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
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
		ch: make(chan *sarama.ProducerMessage, 100),
	}
}

func defaultFilterSpec(spec *Spec) *pipeline.FilterSpec {
	meta := &pipeline.FilterMetaSpec{
		Name:     "kafka-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
	}
	filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, spec)
	return filterSpec
}

func newContext(cid string, topic string, header map[string]string) context.MQTTContext {
	client := &context.MockMQTTClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	ctx := context.NewMQTTContext(stdcontext.Background(), client, packet)
	ctx.PublishPacket().SetAllHeader(header)
	return ctx
}

func TestKafka(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{
		Backend: []string{"localhost:1234"},
	}
	filterSpec := defaultFilterSpec(spec)
	k := &Kafka{}
	assert.Panics(func() { k.Init(filterSpec) }, "kafka should panic for invalid backend")

	kafka := Kafka{
		producer: newMockAsyncProducer(),
		done:     make(chan struct{}),
	}
	mqttCtx := newContext("test", "a/b/c", map[string]string{"a": "1", "b": "2"})

	kafka.HandleMQTT(mqttCtx)
	msg := <-kafka.producer.(*mockAsyncProducer).ch
	assert.Equal(msg.Topic, mqttCtx.PublishPacket().Topic())
	assert.Equal(2, len(msg.Headers))

	mqttCtx = newContext("test", "a/b/c", map[string]string{})
	kafka.HandleMQTT(mqttCtx)
	msg = <-kafka.producer.(*mockAsyncProducer).ch
	assert.Equal(msg.Topic, mqttCtx.PublishPacket().Topic())
	assert.Equal(0, len(msg.Headers))
	kafka.Close()
}
