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
)

// SessionCacheManager is the interface for session cache.
// SessionCache is used in broker mode only. It stores session info of clients from
// different easegress instances. It update topic manager when session info of is updated.
type SessionCacheManager interface {
	update(clients map[string]*SessionInfo)
	delete(clientID string)
	sync(clients map[string]*SessionInfo)
	getEGName(clientID string) string
	close()
}

type sessionCacheManager struct {
	egName      string
	topicMgr    TopicManager
	cache       map[string]*SessionInfo
	egNameCache sync.Map
	writeCh     chan *sessionCacheOp
	doneCh      chan struct{}
}

var _ SessionCacheManager = (*sessionCacheManager)(nil)

type sessionCacheOpType int

const (
	sessionCacheOpUpdate sessionCacheOpType = iota
	sessionCacheOpDelete
	sessionCacheOpSync
)

type sessionCacheOp struct {
	opType   sessionCacheOpType
	clientID string
	clients  map[string]*SessionInfo
}

func newSessionCacheManager(sepc *Spec, topicMgr TopicManager) SessionCacheManager {
	mgr := &sessionCacheManager{
		egName:   sepc.EGName,
		topicMgr: topicMgr,
		cache:    make(map[string]*SessionInfo),
		writeCh:  make(chan *sessionCacheOp, 10000),
		doneCh:   make(chan struct{}),
	}
	go mgr.run()
	return mgr
}

func (c *sessionCacheManager) update(clients map[string]*SessionInfo) {
	c.writeCh <- &sessionCacheOp{
		opType:  sessionCacheOpUpdate,
		clients: clients,
	}
}

func (c *sessionCacheManager) delete(clientID string) {
	c.writeCh <- &sessionCacheOp{
		opType:   sessionCacheOpDelete,
		clientID: clientID,
	}
}

func (c *sessionCacheManager) sync(clients map[string]*SessionInfo) {
	c.writeCh <- &sessionCacheOp{
		opType:  sessionCacheOpSync,
		clients: clients,
	}
}

func (c *sessionCacheManager) processUpdate(clients map[string]*SessionInfo) {
	for k, v := range clients {
		c.processSingleUpdate(k, v)
	}
}

func (c *sessionCacheManager) processSingleUpdate(clientID string, session *SessionInfo) {
	c.egNameCache.Store(clientID, session.EGName)
	if session.EGName == c.egName {
		c.cache[clientID] = session
		return
	}

	oldSession, ok := c.cache[clientID]
	c.cache[clientID] = session
	if !ok {
		topics := make([]string, 0, len(session.Topics))
		qoss := make([]byte, 0, len(session.Topics))
		for t, v := range session.Topics {
			topics = append(topics, t)
			qoss = append(qoss, byte(v))
		}
		c.topicMgr.subscribe(topics, qoss, clientID)
		return
	}

	subTopics := []string{}
	subQoss := []byte{}
	for k, v := range session.Topics {
		if _, ok := oldSession.Topics[k]; !ok {
			subTopics = append(subTopics, k)
			subQoss = append(subQoss, byte(v))
		}
	}
	unsubTopics := []string{}
	for k := range oldSession.Topics {
		if _, ok := session.Topics[k]; !ok {
			unsubTopics = append(unsubTopics, k)
		}
	}
	c.topicMgr.subscribe(subTopics, subQoss, clientID)
	c.topicMgr.unsubscribe(unsubTopics, clientID)
}

func (c *sessionCacheManager) processDelete(clientID string) {
	c.egNameCache.Delete(clientID)
	session, ok := c.cache[clientID]
	if ok {
		topics := make([]string, 0, len(session.Topics))
		for t := range session.Topics {
			topics = append(topics, t)
		}
		c.topicMgr.disconnect(topics, clientID)
		delete(c.cache, clientID)
	}
}

func (c *sessionCacheManager) processSync(clients map[string]*SessionInfo) {
	for k := range c.cache {
		if _, ok := clients[k]; !ok {
			c.processDelete(k)
		}
	}
	for k, v := range clients {
		c.processSingleUpdate(k, v)
	}
}

func (c *sessionCacheManager) run() {
	for {
		select {
		case <-c.doneCh:
			return
		case op := <-c.writeCh:
			switch op.opType {
			case sessionCacheOpUpdate:
				c.processUpdate(op.clients)
			case sessionCacheOpDelete:
				c.processDelete(op.clientID)
			case sessionCacheOpSync:
				c.processSync(op.clients)
			}
		}
	}
}

func (c *sessionCacheManager) getEGName(clientID string) string {
	val, ok := c.egNameCache.Load(clientID)
	if !ok {
		return c.egName
	}
	return val.(string)
}

func (c *sessionCacheManager) close() {
	close(c.doneCh)
}
