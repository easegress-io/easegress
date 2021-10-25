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

package context

import (
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type (
	// MQTTContext is context for MQTT protocol
	MQTTContext interface {
		Context
		Client() MQTTClient
		Topic() string
		Payload() []byte
	}

	// MQTTClient contains client info that send this packet
	MQTTClient interface {
		ClientID() string
	}

	mqttContext struct {
		client MQTTClient
		packet *packets.PublishPacket
	}
)

var _ MQTTContext = (*mqttContext)(nil)

func NewMQTTContext(client MQTTClient, packet *packets.PublishPacket) MQTTContext {
	return &mqttContext{
		client: client,
		packet: packet,
	}
}

func (m *mqttContext) Protocol() Protocol {
	return MQTT
}

func (m *mqttContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (m *mqttContext) Done() <-chan struct{} {
	return nil
}

func (m *mqttContext) Err() error {
	return nil
}

func (m *mqttContext) Value(key interface{}) interface{} {
	return nil
}

func (m *mqttContext) Client() MQTTClient {
	return m.client
}

func (m *mqttContext) Topic() string {
	return m.packet.TopicName
}

func (m *mqttContext) Payload() []byte {
	return m.packet.Payload
}
