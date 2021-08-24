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
		sync.Mutex

		broker    *Broker
		connect   *packets.ConnectPacket
		topics    map[string]byte // map subscribe topic to qos
		clientID  string
		cleanFlag bool

		nextID  uint16
		qos0    []*Message
		qos1    []*Message
		pending map[uint16]*packets.PublishPacket
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

func (s *Session) init(b *Broker, connect *packets.ConnectPacket) error {
	s.broker = b
	s.connect = connect
	s.clientID = connect.ClientIdentifier
	s.cleanFlag = connect.CleanSession
	s.topics = make(map[string]byte)
	s.qos0 = []*Message{}
	s.qos1 = []*Message{}
	s.pending = make(map[uint16]*packets.PublishPacket)
	return nil
}

func (s *Session) update(connect *packets.ConnectPacket) error {
	s.Lock()
	defer s.Unlock()
	if s.connect.CleanSession {
		return fmt.Errorf("session %s set CleanSession, should not update", s.clientID)
	}
	s.connect = connect
	return nil
}

func (s *Session) subscribe(topics []string, qoss []byte) error {
	s.Lock()
	defer s.Unlock()
	for i, t := range topics {
		s.topics[t] = qoss[i]
	}
	return nil
}

func (s *Session) unsubscribe(topics []string) error {
	s.Lock()
	defer s.Unlock()
	for _, t := range topics {
		delete(s.topics, t)
	}
	return nil
}

func (s *Session) allSubscribes() ([]string, []byte, error) {
	s.Lock()
	defer s.Unlock()

	var sub []string
	var qos []byte
	for k, v := range s.topics {
		sub = append(sub, k)
		qos = append(qos, v)
	}
	return sub, qos, nil
}

func (s *Session) getPacketFromMsg(msg *Message) *packets.PublishPacket {
	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.Qos = msg.qos
	p.TopicName = msg.topic
	p.Payload = msg.payload
	p.MessageID = s.nextID
	// the overflow is okay here
	// the session will give unique id from 0 to 65535 and do this again and again
	s.nextID++
	return p
}

func (s *Session) publishQueuedMsg() {
	client := s.broker.getClient(s.clientID)
	if client == nil {
		return
	}
	s.Lock()
	defer s.Unlock()

	ps := []packets.ControlPacket{}
	for _, msg := range s.qos0 {
		p := s.getPacketFromMsg(msg)
		ps = append(ps, p)
	}
	for _, msg := range s.qos1 {
		p := s.getPacketFromMsg(msg)
		ps = append(ps, p)
		s.pending[p.MessageID] = p
	}

	go client.writePackets(ps)
}

func (s *Session) publish(msg *Message) {
	client := s.broker.getClient(s.clientID)
	s.Lock()
	defer s.Unlock()

	if qos, ok := s.topics[msg.topic]; !ok || qos < msg.qos {
		return
	}
	if client == nil {
		if msg.qos == Qos0 {
			s.qos0 = append(s.qos0, msg)
		} else if msg.qos == Qos1 {
			s.qos1 = append(s.qos1, msg)
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	} else {
		if len(s.qos0) != 0 || len(s.qos1) != 0 {
			go s.publishQueuedMsg()
		}
		p := s.getPacketFromMsg(msg)
		if msg.qos == Qos0 {
			go client.writePacket(p)
		} else if msg.qos == Qos1 {
			s.pending[p.MessageID] = p
			go client.writePacket(p)
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	}
}

func (s *Session) puback(p *packets.PubackPacket) {
	s.Lock()
	defer s.Unlock()
	delete(s.pending, p.MessageID)
}

func (s *Session) cleanSession() bool {
	return s.cleanFlag
}
