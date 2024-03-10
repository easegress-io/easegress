/*
 * Copyright (c) 2017, The Easegress Authors
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
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/codectool"

	"github.com/openzipkin/zipkin-go/model"
)

const defaultRetryInterval = 30 * time.Second

type (
	// SessionInfo is info about session that will be put into etcd for persistency
	SessionInfo struct {
		// map subscribe topic to qos
		EGName    string         `json:"egName"`
		Name      string         `json:"name"`
		Topics    map[string]int `json:"topics"`
		ClientID  string         `json:"clientID"`
		CleanFlag bool           `json:"cleanFlag"`
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
		// retry Qos1 packet
		retryInterval time.Duration
		refreshStore  atomic.Value
	}

	// Message is the message send from broker to client
	Message struct {
		Topic      string `json:"topic"`
		B64Payload string `json:"b64Payload"`
		QoS        int    `json:"qos"`
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
	if swapped := s.refreshStore.CompareAndSwap(true, false); !swapped {
		return
	}

	ss := func() *SessionStore {
		s.Lock()
		str, err := s.encode()
		s.Unlock()
		if err != nil {
			logger.Errorf("encode session %+v failed: %v", s, err)
			return nil
		}

		return &SessionStore{
			key:   s.info.ClientID,
			value: str,
		}
	}()
	if ss == nil {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	select {
	case s.storeCh <- *ss:
		logger.Infof("session: %s  store:%s", s.info.ClientID, ss.value)
		return
	case <-ticker.C:
		logger.Infof("session: %s  store:%s, timeout", s.info.ClientID, ss.value)
		s.refreshStore.Store(true)
		return
	}
}

func (s *Session) encode() (string, error) {
	b, err := codectool.MarshalJSON(s.info)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Session) decode(str string) error {
	if s.info == nil {
		s.info = &SessionInfo{}
	}
	return codectool.UnmarshalJSON([]byte(str), s.info)
}

func (s *Session) init(sm *SessionManager, b *Broker, connect *packets.ConnectPacket) error {
	s.broker = b
	s.storeCh = sm.storeCh
	s.done = make(chan struct{})
	s.pending = make(map[uint16]*Message)
	s.pendingQueue = []uint16{}
	s.retryInterval = time.Second * time.Duration(b.spec.RetryInterval)

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
	// s.store()
	s.refreshStore.Store(true)
	s.Unlock()
}

func (s *Session) subscribe(topics []string, qoss []byte) error {
	logger.SpanDebugf(nil, "session %s sub %v", s.info.ClientID, topics)
	s.Lock()
	for i, t := range topics {
		s.info.Topics[t] = int(qoss[i])
	}
	// s.store()
	s.refreshStore.Store(true)
	s.Unlock()
	return nil
}

func (s *Session) unsubscribe(topics []string) error {
	logger.SpanDebugf(nil, "session %s unsub %v", s.info.ClientID, topics)
	s.Lock()
	for _, t := range topics {
		delete(s.info.Topics, t)
	}
	// s.store()
	s.refreshStore.Store(true)
	s.Unlock()
	return nil
}

func (s *Session) allSubscribes() ([]string, []byte, error) {
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

func (s *Session) publish(span *model.SpanContext, client *Client, topic string, payload []byte, qos byte) {
	p := func() packets.ControlPacket {
		s.Lock()
		defer s.Unlock()
		logger.SpanDebugf(span, "session %v publish %v", s.info.ClientID, topic)
		p := s.getPacketFromMsg(topic, payload, qos)
		if qos == QoS1 {
			msg := newMsg(topic, payload, qos)
			s.pending[p.MessageID] = msg
			s.pendingQueue = append(s.pendingQueue, p.MessageID)
		}
		return p
	}()

	if qos == QoS2 {
		logger.SpanErrorf(span, "publish message with qos=2 is not supported currently")
		return
	}
	client.writePacket(p)
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
	msg, messageID := func() (msg *Message, messageID uint16) {
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
				msg = val
				messageID = idx
				break
			}
		}
		return
	}()

	if msg == nil {
		return
	}

	p := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	p.Qos = byte(msg.QoS)
	p.TopicName = msg.Topic
	payload, err := base64.StdEncoding.DecodeString(msg.B64Payload)
	if err != nil {
		logger.Errorf("client:%s, base64 decode error for Message B64Payload %s ", s.info.ClientID, err)
		return
	}
	p.Payload = payload
	p.MessageID = messageID
	if client != nil {
		client.writePacket(p)
	} else {
		logger.Warnf("client %v do resend but client is nil, ignored", s.info.ClientID)
	}
}

// backgroundSessionTask process two tasks peroidly:
// - task 1: sync the session information to global etcd store
// - task 2: resend the packet to the subscriber with QoS 1 or 2
func (s *Session) backgroundSessionTask() {
	if s.retryInterval <= 0 {
		logger.Warnf("invalid s.retryInterval :%d, mandatory setting to 30s", s.retryInterval)
		s.retryInterval = defaultRetryInterval
	}
	resendTime := time.Now().Add(s.retryInterval)
	debugLogTime := time.Now().Add(time.Minute)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.store()
			if time.Now().After(resendTime) {
				s.doResend()
				resendTime = time.Now().Add(s.retryInterval)
			}
		}
		if time.Now().After(debugLogTime) {
			logger.SpanDebugf(nil, "session %v resend", s.info.ClientID)
			debugLogTime = time.Now().Add(time.Minute)
		}
	}
}
