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
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/logger"
)

type (
	// SessionManager manage the status of session for clients
	SessionManager struct {
		broker     *Broker
		sessionMap sync.Map
		store      storage
		storeCh    chan SessionStore
		done       chan struct{}
	}

	// SessionStore for session store, key is session clientID, value is session json marshal value
	SessionStore struct {
		key   string
		value string
	}
)

func newSessionManager(b *Broker, store storage) *SessionManager {
	sm := &SessionManager{
		broker:  b,
		store:   store,
		storeCh: make(chan SessionStore, 1024), // change to unbounded buffer, due to avoiding to block write operation
		done:    make(chan struct{}),
	}
	go sm.doStore()
	return sm
}

func (sm *SessionManager) close() {
	close(sm.done)
}

func (sm *SessionManager) doStore() {
	for {
		select {
		case <-sm.done:
			return
		case kv := <-sm.storeCh:
			logger.SpanDebugf(nil, "session manager store session %v", kv.key)
			err := sm.store.put(sessionStoreKey(kv.key), kv.value)
			if err != nil {
				logger.SpanErrorf(nil, "put session %v into storage failed: %v", kv.key, err)
			}
		}
	}
}

func (sm *SessionManager) newSessionFromConn(connect *packets.ConnectPacket) *Session {
	s := &Session{}
	s.init(sm, sm.broker, connect)
	sm.sessionMap.Store(connect.ClientIdentifier, s)
	go s.backgroundSessionTask()
	return s
}

func (sm *SessionManager) newSessionFromJSON(str *string) *Session {
	sess := &Session{}
	sess.broker = sm.broker
	sess.storeCh = sm.storeCh
	sess.done = make(chan struct{})
	sess.pending = make(map[uint16]*Message)
	sess.pendingQueue = []uint16{}

	sess.info = &SessionInfo{}
	err := sess.decode(*str)
	if err != nil {
		return nil
	}
	go sess.backgroundSessionTask()
	return sess
}

func (sm *SessionManager) get(clientID string) *Session {
	if val, ok := sm.sessionMap.Load(clientID); ok {
		return val.(*Session)
	}

	str, err := sm.store.get(sessionStoreKey(clientID))
	if err != nil || str == nil {
		return nil
	}

	sess := sm.newSessionFromJSON(str)
	if sess != nil {
		sm.sessionMap.Store(sess.info.ClientID, sess)
	}
	return sess
}

func (sm *SessionManager) delLocal(clientID string) bool {
	if val, ok := sm.sessionMap.LoadAndDelete(clientID); ok {
		sess := val.(*Session)
		sess.close()
		return true
	}
	return false
}

func (sm *SessionManager) delDB(clientID string) {
	err := sm.store.delete(sessionStoreKey(clientID))
	if err != nil {
		logger.SpanErrorf(nil, "delete session %v failed, %v", err)
	}
}
