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
	"sync"

	"github.com/Shopify/sarama"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	// BackendMQ is backend message queue for MQTT proxy
	backendMQ interface {
		publish(p *packets.PublishPacket) error
		Publish(target string, data []byte, headers map[string]string) error
		close()
	}

	// KafkaMQ is backend message queue for MQTT proxy by using Kafka
	KafkaMQ struct {
		producer sarama.AsyncProducer
		mapFunc  topicMapFunc
		done     chan struct{}
	}

	testMQ struct {
		mu  sync.Mutex
		ch  chan *packets.PublishPacket
		msg map[string]map[string]string
	}
)

var _ backendMQ = (*KafkaMQ)(nil)
var _ backendMQ = (*testMQ)(nil)
var _ context.MQTTBackend = (*KafkaMQ)(nil)
var _ context.MQTTBackend = (*testMQ)(nil)

const (
	kafkaType  = "Kafka"
	testMQType = "TestMQ"
)

func newBackendMQ(spec *Spec) backendMQ {
	switch spec.BackendType {
	case kafkaType:
		return newKafkaMQ(spec)
	case testMQType:
		t := &testMQ{}
		t.ch = make(chan *packets.PublishPacket, 100)
		t.msg = make(map[string]map[string]string)
		return t
	default:
		logger.SpanErrorf(nil, "backend type <%s> not support", spec.BackendType)
		return nil
	}
}

func newKafkaMQ(spec *Spec) backendMQ {
	k := &KafkaMQ{}
	k.mapFunc = getTopicMapFunc(spec.TopicMapper)
	k.done = make(chan struct{})

	config := sarama.NewConfig()
	config.ClientID = spec.Name
	config.Version = sarama.V1_0_0_0
	producer, err := sarama.NewAsyncProducer(spec.Kafka.Backend, config)
	if err != nil {
		logger.SpanErrorf(nil, "start sarama producer with address %v failed: %v", spec.Kafka.Backend, err)
		return nil
	}

	go func() {
		for {
			select {
			case <-k.done:
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.SpanErrorf(nil, "sarama producer failed: %v", err)
			}
		}
	}()

	k.producer = producer
	return k
}

func (k *KafkaMQ) publish(p *packets.PublishPacket) error {
	var msg *sarama.ProducerMessage
	logger.SpanDebugf(nil, "produce msg with topic %s", p.TopicName)

	if k.mapFunc != nil {
		topic, headers, err := k.mapFunc(p.TopicName)
		if err != nil {
			return fmt.Errorf("packet TopicName %s not match TopicMapper rules", p.TopicName)
		}
		kafkaHeaders := []sarama.RecordHeader{}
		for k, v := range headers {
			kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
		}

		msg = &sarama.ProducerMessage{
			Topic:   topic,
			Headers: kafkaHeaders,
			Value:   sarama.ByteEncoder(p.Payload),
		}
	} else {
		msg = &sarama.ProducerMessage{
			Topic: p.TopicName,
			Value: sarama.ByteEncoder(p.Payload),
		}
	}
	k.producer.Input() <- msg
	return nil
}

func (k *KafkaMQ) close() {
	close(k.done)
	err := k.producer.Close()
	if err != nil {
		logger.SpanErrorf(nil, "close kafka producer failed: %v", err)
	}
}

// Publish publish msg to Kafka backend
func (k *KafkaMQ) Publish(target string, data []byte, headers map[string]string) error {
	var msg *sarama.ProducerMessage
	kafkaHeaders := make([]sarama.RecordHeader, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}
	msg = &sarama.ProducerMessage{
		Topic:   target,
		Headers: kafkaHeaders,
		Value:   sarama.ByteEncoder(data),
	}
	k.producer.Input() <- msg
	return nil
}

func (t *testMQ) publish(p *packets.PublishPacket) error {
	t.ch <- p
	return nil
}

// Publish publish msg to testMQ backend
func (t *testMQ) Publish(target string, data []byte, headers map[string]string) error {
	t.mu.Lock()
	t.msg[target] = headers
	t.mu.Unlock()
	return nil
}

func (t *testMQ) close() {
}
