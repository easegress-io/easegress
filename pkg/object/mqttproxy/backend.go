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
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	BackendMQ interface {
		publish(p *packets.PublishPacket) error
	}

	KafkaMQ struct {
	}

	testMQ struct {
		ch chan *packets.PublishPacket
	}
)

const (
	kafkaType  = "Kafka"
	testMQType = "TestMQ"
)

func (k *KafkaMQ) publish(p *packets.PublishPacket) error {
	return nil
}

func (t *testMQ) publish(p *packets.PublishPacket) error {
	t.ch <- p
	return nil
}

func newBackendMQ(spec *Spec) BackendMQ {
	switch spec.BackendType {
	case kafkaType:
		return &KafkaMQ{}
	case testMQType:
		t := &testMQ{}
		t.ch = make(chan *packets.PublishPacket, 10)
		return t
	default:
		logger.Errorf("backend type not support %s", spec.BackendType)
	}
	return nil
}
