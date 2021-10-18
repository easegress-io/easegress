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
	"time"

	"gopkg.in/yaml.v2"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type (
	// SessionInfo is info about session that will be put into etcd for persistency
	SessionInfo struct {
		// map subscribe topic to qos
		EGName    string         `yaml:"egName"`
		Name      string         `yaml:"name"`
		Topics    map[string]int `yaml:"topics"`
		ClientID  string         `yaml:"clientID"`
		CleanFlag bool           `yaml:"cleanFlag"`
	}

	// Session includes the information about the connect between client and broker,
	// such as topic subscribe, not-send messages, etc.
	Session struct {
		sync.Mutex
		broker       *Broker
		storeCh      chan SessionStore
		info         *SessionInfo
		done         chan struct{}
		pending      map[uint16]*Message
		pendingQueue []uint16
		nextID       uint16
	}

	// Message is the message send from broker to client
	Message struct {
		Topic      string `yaml:"topic"`
		B64Payload string `yaml:"b64Payload"`
		QoS        int    `yaml:"qos"`
	}
)

func newMsg(topic string, payload []byte, qos byte) *Message {
	m := &Message{
		Topic:      topic,
		B64Payload: base64.StdEncoding.EncodeToString(payload),
		QoS:        int(qos),
	}
	return m
}

func (s *Session) store() {
	logger.Debugf("session %v store", s.info.ClientID)
	str, err := s.encode()
	if err != nil {
		logger.Errorf("encode session %+v failed: %v", s, err)
		return
	}
	ss := SessionStore{
		key:   s.info.ClientID,
		value: str,
	}
	go func() {
		s.storeCh <- ss
	}()
}

func (s *Session) encode() (string, error) {
	b, err := yaml.Marshal(s.info)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Session) decode(str string) error {
	return yaml.Unmarshal([]byte(str), s.info)
}

func (s *Session) init(sm *SessionManager, b *Broker, connect *packets.ConnectPacket) error {
	s.broker = b
	s.storeCh = sm.storeCh
	s.done = make(chan struct{})
	s.pending = make(map[uint16]*Message)
	s.pendingQueue = []uint16{}

	s.info = &SessionInfo{}
	s.info.EGName = b.egName
	s.info.Name = b.name
	s.info.ClientID = connect.ClientIdentifier
	s.info.CleanFlag = connect.CleanSession
	s.info.Topics = make(map[string]int)
	return nil
}

func (s *Session) updateEGName(egName, name string) {
	s.Lock()
	s.info.EGName = egName
	s.info.Name = name
	s.store()
	s.Unlock()
}

func (s *Session) subscribe(topics []string, qoss []byte) error {
	logger.Debugf("session %s sub %v", s.info.ClientID, topics)
	s.Lock()
	for i, t := range topics {
		s.info.Topics[t] = int(qoss[i])
	}
	s.store()
	s.Unlock()
	return nil
}

func (s *Session) unsubscribe(topics []string) error {
	logger.Debugf("session %s unsub %v", s.info.ClientID, topics)
	s.Lock()
	for _, t := range topics {
		delete(s.info.Topics, t)
	}
	s.store()
	s.Unlock()
	return nil
}

func (s *Session) allSubscribes() ([]string, []byte, error) {
	logger.Debugf("session %s all sub", s.info.ClientID)
	s.Lock()

	var sub []string
	var qos []byte
	for k, v := range s.info.Topics {
		sub = append(sub, k)
		qos = append(qos, byte(v))
	}
	s.Unlock()
	return sub, qos, nil
}

func (s *Session) getPacketFromMsg(topic string, payload []byte, qos byte) *packets.PublishPacket {
	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.Qos = qos
	p.TopicName = topic
	p.Payload = payload
	p.MessageID = s.nextID
	// the overflow is okay here
	// the session will give unique id from 0 to 65535 and do this again and again
	s.nextID++
	return p
}

func (s *Session) publish(topic string, payload []byte, qos byte) {
	client := s.broker.getClient(s.info.ClientID)
	if client == nil {
		logger.Errorf("client %s is offline", s.info.ClientID)
		return
	}

	s.Lock()
	defer s.Unlock()

	logger.Debugf("session %v publish %v", s.info.ClientID, topic)
	p := s.getPacketFromMsg(topic, payload, qos)
	if qos == QoS0 {
		select {
		case client.writeCh <- p:
		default:
		}
	} else if qos == QoS1 {
		msg := newMsg(topic, payload, qos)
		s.pending[p.MessageID] = msg
		s.pendingQueue = append(s.pendingQueue, p.MessageID)
		client.writePacket(p)
	} else {
		logger.Errorf("publish message with qos=2 is not supported currently")
	}
}

func (s *Session) puback(p *packets.PubackPacket) {
	s.Lock()
	delete(s.pending, p.MessageID)
	s.Unlock()
}

func (s *Session) cleanSession() bool {
	return s.info.CleanFlag
}

func (s *Session) close() {
	close(s.done)
}

func (s *Session) doResend() {
	client := s.broker.getClient(s.info.ClientID)
	s.Lock()
	defer s.Unlock()

	if len(s.pending) == 0 {
		s.pendingQueue = []uint16{}
		return
	}
	for i, idx := range s.pendingQueue {
		if val, ok := s.pending[idx]; ok {
			// find first msg need to resend
			s.pendingQueue = s.pendingQueue[i:]
			p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			p.Qos = byte(val.QoS)
			p.TopicName = val.Topic
			payload, err := base64.StdEncoding.DecodeString(val.B64Payload)
			if err != nil {
				logger.Errorf("base64 decode error for Message B64Payload %s", err)
				return
			}
			p.Payload = payload
			p.MessageID = idx
			if client != nil {
				client.writePacket(p)
			} else {
				logger.Debugf("session %v do resend but client is nil", s.info.ClientID)
			}
			return
		}
	}
}

func (s *Session) backgroundResendPending() {
	debugLogTime := time.Now().Add(time.Minute)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.doResend()
		}
		if time.Now().After(debugLogTime) {
			logger.Debugf("session %v resend", s.info.ClientID)
			debugLogTime = time.Now().Add(time.Minute)
		}
	}
}
