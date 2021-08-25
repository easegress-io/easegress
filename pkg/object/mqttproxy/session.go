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
	"encoding/base64"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	// SessionManager manage the status of session for clients
	SessionManager struct {
		smap sync.Map
	}

	// Session includes the information about the connect between client and broker,
	// such as topic subscribe, not-send messages, etc.
	Session struct {
		sync.Mutex `yaml:"-"`
		broker     *Broker `yaml:"-"`

		// map subscribe topic to qos
		Topics    map[string]int `yaml:"topics"`
		ClientID  string         `yaml:"clientID"`
		CleanFlag bool           `yaml:"cleanFlag"`

		NextID  uint16              `yaml:"nextID"`
		Qos1    []*Message          `yaml:"qos1"`
		Pending map[uint16]*Message `yaml:"pending"`
	}

	// Message is the message send from broker to client
	Message struct {
		Topic      string `yaml:"topic"`
		B64Payload string `yaml:"b64Payload"`
		Qos        int    `yaml:"qos"`
	}
)

func newSessionManager() *SessionManager {
	s := &SessionManager{}
	return s
}

func (sm *SessionManager) new(b *Broker, connect *packets.ConnectPacket) *Session {
	s := &Session{}
	s.init(b, connect)
	sm.smap.Store(connect.ClientIdentifier, s)
	return s
}

func (sm *SessionManager) get(clientID string) *Session {
	if val, ok := sm.smap.Load(clientID); ok {
		return val.(*Session)
	}
	return nil
}

func (sm *SessionManager) del(clientID string) {
	sm.smap.Delete(clientID)
}

func (s *Session) encode() (string, error) {
	b, err := yaml.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Session) decode(str string) error {
	return yaml.Unmarshal([]byte(str), s)
}

func (s *Session) init(b *Broker, connect *packets.ConnectPacket) error {
	s.broker = b
	s.ClientID = connect.ClientIdentifier
	s.CleanFlag = connect.CleanSession
	s.Topics = make(map[string]int)
	s.Qos1 = []*Message{}
	s.Pending = make(map[uint16]*Message)
	return nil
}

func (s *Session) subscribe(topics []string, qoss []byte) error {
	s.Lock()
	defer s.Unlock()
	for i, t := range topics {
		s.Topics[t] = int(qoss[i])
	}
	return nil
}

func (s *Session) unsubscribe(topics []string) error {
	s.Lock()
	defer s.Unlock()
	for _, t := range topics {
		delete(s.Topics, t)
	}
	return nil
}

func (s *Session) allSubscribes() ([]string, []byte, error) {
	s.Lock()
	defer s.Unlock()

	var sub []string
	var qos []byte
	for k, v := range s.Topics {
		sub = append(sub, k)
		qos = append(qos, byte(v))
	}
	return sub, qos, nil
}

func (s *Session) getPacketFromMsg(topic string, payload []byte, qos byte) *packets.PublishPacket {
	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.Qos = qos
	p.TopicName = topic
	p.Payload = payload
	p.MessageID = s.NextID
	// the overflow is okay here
	// the session will give unique id from 0 to 65535 and do this again and again
	s.NextID++
	return p
}

func (s *Session) publishQueuedMsg() {
	client := s.broker.getClient(s.ClientID)
	if client == nil {
		return
	}
	s.Lock()
	defer s.Unlock()

	ps := []packets.ControlPacket{}
	for _, msg := range s.Qos1 {
		payload, _ := base64.StdEncoding.DecodeString(msg.B64Payload)
		p := s.getPacketFromMsg(msg.Topic, payload, byte(msg.Qos))
		ps = append(ps, p)
		s.Pending[p.MessageID] = msg
	}

	go client.writePackets(ps)
}

func getMsg(topic string, payload []byte, qos byte) *Message {
	m := &Message{
		Topic:      topic,
		B64Payload: base64.StdEncoding.EncodeToString(payload),
		Qos:        int(qos),
	}
	return m
}

func (s *Session) publish(topic string, payload []byte, qos byte) {
	client := s.broker.getClient(s.ClientID)
	s.Lock()
	defer s.Unlock()

	if q, ok := s.Topics[topic]; !ok || byte(q) < qos {
		return
	}
	if client == nil {
		if qos == Qos1 {
			s.Qos1 = append(s.Qos1, getMsg(topic, payload, qos))
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	} else {
		if len(s.Qos1) != 0 {
			go s.publishQueuedMsg()
		}
		p := s.getPacketFromMsg(topic, payload, qos)
		if qos == Qos0 {
			go client.writePacket(p)
		} else if qos == Qos1 {
			msg := getMsg(topic, payload, qos)
			s.Pending[p.MessageID] = msg
			go client.writePacket(p)
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	}
}

func (s *Session) puback(p *packets.PubackPacket) {
	s.Lock()
	defer s.Unlock()
	delete(s.Pending, p.MessageID)
}

func (s *Session) cleanSession() bool {
	return s.CleanFlag
}
