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
	"fmt"
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
		Topics    map[string]int `yaml:"topics"`
		ClientID  string         `yaml:"clientID"`
		CleanFlag bool           `yaml:"cleanFlag"`

		NextID       uint16              `yaml:"nextID"`
		Qos1         []*Message          `yaml:"qos1"`
		Pending      map[uint16]*Message `yaml:"pending"`
		PendingQueue []uint16            `yaml:"pendingQueue"`
	}

	// Session includes the information about the connect between client and broker,
	// such as topic subscribe, not-send messages, etc.
	Session struct {
		sync.Mutex
		broker  *Broker
		storeCh chan SessionStore
		info    *SessionInfo
		done    chan struct{}
	}

	// Message is the message send from broker to client
	Message struct {
		Topic      string `yaml:"topic"`
		B64Payload string `yaml:"b64Payload"`
		Qos        int    `yaml:"qos"`
	}
)

func getMsg(topic string, payload []byte, qos byte) *Message {
	m := &Message{
		Topic:      topic,
		B64Payload: base64.StdEncoding.EncodeToString(payload),
		Qos:        int(qos),
	}
	return m
}

func (s *Session) store() {
	str, err := s.encode()
	if err != nil {
		logger.Errorf("encode session %+v failed, %v", s, err)
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

func (s *Session) init(b *Broker, connect *packets.ConnectPacket) error {
	s.broker = b
	s.done = make(chan struct{})
	s.info = &SessionInfo{}
	s.info.ClientID = connect.ClientIdentifier
	s.info.CleanFlag = connect.CleanSession
	s.info.Topics = make(map[string]int)
	s.info.Qos1 = []*Message{}
	s.info.Pending = make(map[uint16]*Message)
	s.info.PendingQueue = []uint16{}
	return nil
}

func (s *Session) subscribe(topics []string, qoss []byte) error {
	s.Lock()
	defer s.Unlock()
	for i, t := range topics {
		s.info.Topics[t] = int(qoss[i])
	}
	s.store()
	return nil
}

func (s *Session) unsubscribe(topics []string) error {
	s.Lock()
	defer s.Unlock()
	for _, t := range topics {
		delete(s.info.Topics, t)
	}
	s.store()
	return nil
}

func (s *Session) allSubscribes() ([]string, []byte, error) {
	s.Lock()
	defer s.Unlock()

	var sub []string
	var qos []byte
	for k, v := range s.info.Topics {
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
	p.MessageID = s.info.NextID
	// the overflow is okay here
	// the session will give unique id from 0 to 65535 and do this again and again
	s.info.NextID++
	return p
}

func (s *Session) publishQueuedMsg() {
	client := s.broker.getClient(s.info.ClientID)
	if client == nil {
		return
	}
	s.Lock()
	defer s.Unlock()

	ps := []packets.ControlPacket{}
	for _, msg := range s.info.Qos1 {
		payload, _ := base64.StdEncoding.DecodeString(msg.B64Payload)
		p := s.getPacketFromMsg(msg.Topic, payload, byte(msg.Qos))
		ps = append(ps, p)
		s.info.Pending[p.MessageID] = msg
		s.info.PendingQueue = append(s.info.PendingQueue, p.MessageID)
	}
	go client.writePackets(ps)
}

func (s *Session) publish(topic string, payload []byte, qos byte) {
	client := s.broker.getClient(s.info.ClientID)
	s.Lock()
	defer s.Unlock()

	if q, ok := s.info.Topics[topic]; !ok || byte(q) < qos {
		return
	}
	if client == nil {
		if qos == Qos1 {
			s.info.Qos1 = append(s.info.Qos1, getMsg(topic, payload, qos))
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	} else {
		if len(s.info.Qos1) != 0 {
			go s.publishQueuedMsg()
		}
		p := s.getPacketFromMsg(topic, payload, qos)
		if qos == Qos0 {
			go client.writePacket(p)
		} else if qos == Qos1 {
			msg := getMsg(topic, payload, qos)
			s.info.Pending[p.MessageID] = msg
			s.info.PendingQueue = append(s.info.PendingQueue, p.MessageID)
			go client.writePacket(p)
		} else {
			logger.Errorf("current not support to publish message with qos=2")
		}
	}
	s.store()
}

func (s *Session) puback(p *packets.PubackPacket) {
	s.Lock()
	defer s.Unlock()
	delete(s.info.Pending, p.MessageID)
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

	if len(s.info.Pending) == 0 {
		s.info.PendingQueue = []uint16{}
		return
	}
	for i, idx := range s.info.PendingQueue {
		if val, ok := s.info.Pending[idx]; ok {
			// find first msg need to resend
			s.info.PendingQueue = s.info.PendingQueue[i:]
			p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			p.Qos = byte(val.Qos)
			p.TopicName = val.Topic
			payload, err := base64.StdEncoding.DecodeString(val.B64Payload)
			if err != nil {
				logger.Errorf("base64 decode error for Message B64Payload %s", err)
				fmt.Printf("base64 decode error for Message B64Payload %s", err)
				return
			}
			p.Payload = payload
			p.MessageID = idx
			if client != nil {
				go client.writePacket(p)
			}
			return
		}
	}
}

func (s *Session) backgroundResendPending() {
	for {
		select {
		case <-s.done:
			return
		default:
			s.doResend()
		}
		time.Sleep(100 * time.Millisecond)
	}
}
