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
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/object/function/storage"
)

type (
	// SessionManager manage the status of session for clients
	SessionManager struct {
		broker  *Broker
		smap    sync.Map
		store   storage.Storage
		storeCh chan SessionStore
		done    chan struct{}
	}

	// SessionStore for session store, key is session clientID, value is session yaml marshal value
	SessionStore struct {
		key   string
		value string
	}
)

func newSessionManager(b *Broker, store storage.Storage) *SessionManager {
	sm := &SessionManager{
		broker:  b,
		store:   store,
		storeCh: make(chan SessionStore),
		done:    make(chan struct{}),
	}
	go sm.doStore()

	// get store session and init sessMgr and topicMgr
	store.Lock()
	allSess, err := store.GetPrefix(sessionStoreKey(""))
	store.Unlock()
	if err != nil {
		return sm
	}
	for _, v := range allSess {
		sess := sm.newSessionFromYaml(&v)
		// init topicMgr here too
		topics := []string{}
		for k := range sess.info.Topics {
			topics = append(topics, k)
		}
		b.topicMgr.subscribe(topics, sess.info.ClientID)
		sm.smap.Store(sess.info.ClientID, sess)
	}
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
			sm.store.Lock()
			sm.store.Put(sessionStoreKey(kv.key), kv.value)
			sm.store.Unlock()
		}
	}
}

func (sm *SessionManager) newSessionFromConn(b *Broker, connect *packets.ConnectPacket) *Session {
	s := &Session{}
	s.storeCh = sm.storeCh
	s.init(b, connect)
	sm.smap.Store(connect.ClientIdentifier, s)
	go s.resendPending()
	return s
}

func (sm *SessionManager) newSessionFromYaml(str *string) *Session {
	sess := &Session{}
	sess.broker = sm.broker
	sess.storeCh = sm.storeCh
	sess.info = &SessionInfo{}
	sess.done = make(chan struct{})
	err := sess.decode(*str)
	if err != nil {
		return nil
	}
	go sess.resendPending()
	return sess
}

func (sm *SessionManager) get(clientID string) *Session {
	if val, ok := sm.smap.Load(clientID); ok {
		return val.(*Session)
	}

	sm.store.Lock()
	defer sm.store.Unlock()
	str, err := sm.store.Get(sessionStoreKey(clientID))
	if err != nil || str == nil {
		return nil
	}

	sess := sm.newSessionFromYaml(str)
	if sess != nil {
		sm.smap.Store(sess.info.ClientID, sess)
	}
	return sess
}

func (sm *SessionManager) delLocal(clientID string) {
	sm.smap.Delete(clientID)
}
