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

import "github.com/eclipse/paho.mqtt.golang/packets"

type (

	// MQTTPublishPacket defines publish packet for MQTTContext
	MQTTPublishPacket interface {
		Topic() string
		SetTopic(string)

		GetHeader(string) string
		SetAllHeader(map[string]string)
		SetHeader(string, string)
		VisitAllHeader(func(string, string))

		Payload() []byte
	}

	mqttPublishPacket struct {
		topic   string
		header  map[string]string
		payload []byte
	}
)

var _ MQTTPublishPacket = (*mqttPublishPacket)(nil)

func newMQTTPublishPacket(packet *packets.PublishPacket) MQTTPublishPacket {
	publish := &mqttPublishPacket{
		topic:   packet.TopicName,
		header:  make(map[string]string),
		payload: packet.Payload,
	}
	return publish
}

// Topic return topic of mqttPublishPacket
func (p *mqttPublishPacket) Topic() string {
	return p.topic
}

// SetTopic set topic of mqttPublishPacket
func (p *mqttPublishPacket) SetTopic(topic string) {
	p.topic = topic
}

// GetHeader get header of given key, if key not exist, return empty string
func (p *mqttPublishPacket) GetHeader(key string) string {
	if value, ok := p.header[key]; ok {
		return value
	}
	return ""
}

// SetAllHeader set header of mqttPublishPacket
func (p *mqttPublishPacket) SetAllHeader(header map[string]string) {
	p.header = header
}

// SetHeader add key, value pair to mqttPublishPacket header
func (p *mqttPublishPacket) SetHeader(key, value string) {
	p.header[key] = value
}

// VisitAllHeader run function to all header
func (p *mqttPublishPacket) VisitAllHeader(fn func(key string, value string)) {
	for k, v := range p.header {
		fn(k, v)
	}
}

// Payload return payload of mqttPublishPacket
func (p *mqttPublishPacket) Payload() []byte {
	return p.payload
}
