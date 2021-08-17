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
	"github.com/Shopify/sarama"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	BackendMQ interface {
		publish(p *packets.PublishPacket) error
		close()
	}

	KafkaMQ struct {
		producer sarama.AsyncProducer
		done     chan struct{}
	}

	testMQ struct {
		ch chan *packets.PublishPacket
	}
)

const (
	kafkaType  = "Kafka"
	testMQType = "TestMQ"
)

func newBackendMQ(spec *Spec) BackendMQ {
	switch spec.BackendType {
	case kafkaType:
		return newKafkaMQ(spec)
	case testMQType:
		t := &testMQ{}
		t.ch = make(chan *packets.PublishPacket, 10)
		return t
	default:
		logger.Errorf("backend type not support %s", spec.BackendType)
		return nil
	}
}

func newKafkaMQ(spec *Spec) *KafkaMQ {
	k := &KafkaMQ{}
	k.done = make(chan struct{})

	config := sarama.NewConfig()
	config.ClientID = spec.Name
	config.Version = sarama.V0_10_2_0
	producer, err := sarama.NewAsyncProducer(spec.Kafka.Backend, config)
	if err != nil {
		logger.Errorf("start sarama producer failed, broker: %s, err: %s", spec.Kafka.Backend, err)
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
				logger.Errorf("produce failed: %s", err)
			}
		}
	}()

	k.producer = producer
	return k
}

func (k *KafkaMQ) publish(p *packets.PublishPacket) error {
	k.producer.Input() <- &sarama.ProducerMessage{
		Topic: p.TopicName,
		Value: sarama.ByteEncoder(p.Payload),
	}
	return nil
}

func (k *KafkaMQ) close() {
	close(k.done)
	err := k.producer.Close()
	if err != nil {
		logger.Errorf("close kafka producer failed: %s", err)
	}
}

func (t *testMQ) publish(p *packets.PublishPacket) error {
	t.ch <- p
	return nil
}

func (t *testMQ) close() {
}
