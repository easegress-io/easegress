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
		initFlag bool

		connect   *packets.ConnectPacket
		topics    map[string]byte // map subscribe topic to qos
		clientID  string
		cleanFlag bool
	}
)

func newSessionManager() *SessionManager {
	s := &SessionManager{}
	return s
}

func (sm *SessionManager) new(connect *packets.ConnectPacket) *Session {
	s := &Session{}
	s.init(connect)
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

func (s *Session) init(connect *packets.ConnectPacket) error {
	s.connect = connect
	s.clientID = connect.ClientIdentifier
	s.initFlag = true
	s.cleanFlag = connect.CleanSession
	s.topics = make(map[string]byte)
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

func (s *Session) cleanSession() bool {
	return s.cleanFlag
}
