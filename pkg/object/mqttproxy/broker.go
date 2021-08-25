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
	"net"
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/function/storage"
)

type (
	// Broker is MQTT server, will manage client, topic, session, etc.
	Broker struct {
		sync.RWMutex
		name string
		spec *Spec

		listener net.Listener
		backend  BackendMQ
		clients  map[string]*Client
		auth     map[string]string

		sessMgr  *SessionManager
		topicMgr *TopicManager

		// done is the channel for shutdowning this proxy.
		done chan struct{}
	}
)

func newBroker(spec *Spec, store storage.Storage) *Broker {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", spec.Port))
	if err != nil {
		logger.Errorf("%#v gen mqtt tcp listener, failed: %v", spec, err)
		return nil
	}

	broker := &Broker{
		name:     spec.Name,
		spec:     spec,
		listener: l,
		backend:  newBackendMQ(spec),
		clients:  make(map[string]*Client),
		auth:     make(map[string]string),
		topicMgr: newTopicManager(),
		done:     make(chan struct{}),
	}
	broker.sessMgr = newSessionManager(broker, store)
	for _, a := range spec.Auth {
		broker.auth[a.Username] = a.B64Passwd
	}

	go broker.run()
	return broker
}

func (b *Broker) str() string {
	return fmt.Sprintf("%s cfg:%#v", b.name, b.spec)
}

func (b *Broker) run() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			select {
			case <-b.done:
				return
			default:
				logger.Errorf("%s net listener accept err: %s", b.str(), err)
			}
		} else {
			go b.handleConn(conn)
		}
	}
}

func (b *Broker) checkClientAuth(connect *packets.ConnectPacket) bool {
	// do auth here use
	// TODO add real client auth here
	// mock here
	cid := connect.ClientIdentifier
	name := connect.Username
	b64passwd := base64.StdEncoding.EncodeToString(connect.Password)
	if authpasswd, ok := b.auth[name]; ok {
		return (cid != "") && (b64passwd == authpasswd)
	}
	return false
}

func (b *Broker) handleConn(conn net.Conn) {
	defer conn.Close()
	packet, err := packets.ReadPacket(conn)
	if err != nil {
		logger.Errorf("%s read connect packet error: %s", b.str(), err)
		return
	}
	connect, ok := packet.(*packets.ConnectPacket)
	if !ok {
		logger.Errorf("received %s that was not Connect", packet.String())
		return
	}

	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	connack.SessionPresent = connect.CleanSession
	connack.ReturnCode = connect.Validate()
	if connack.ReturnCode != packets.Accepted {
		err = connack.Write(conn)
		logger.Errorf("%s, invalid connection %#v, write back err %s", b.str(), connack.ReturnCode, err)
		return
	}

	if !b.checkClientAuth(connect) {
		connack.ReturnCode = packets.ErrRefusedNotAuthorised
		err = connack.Write(conn)
		logger.Errorf("%s, invalid connection %#v, write back err %s", b.str(), connack.ReturnCode, err)
		return
	}

	err = connack.Write(conn)
	if err != nil {
		logger.Errorf("%s, send connack to client %s err %s", b.str(), connect.ClientIdentifier, err)
		return
	}

	client := newClient(connect, b, conn)
	cid := client.info.cid

	b.Lock()
	if oldClient, ok := b.clients[cid]; ok {
		oldClient.close()
	}
	b.clients[client.info.cid] = client
	b.setSession(client, connect)
	b.Unlock()

	// update session
	client.session.publishQueuedMsg()
	client.readLoop()
}

func (b *Broker) setSession(client *Client, connect *packets.ConnectPacket) {
	// when clean session is false, previous session exist and previous session not clean session,
	// then we use previous session, otherwise use new session
	prevSess := b.sessMgr.get(connect.ClientIdentifier)
	if !connect.CleanSession && (prevSess != nil) && !prevSess.cleanSession() {
		// prevSess.update(connect)
		client.session = prevSess
	} else {
		client.session = b.sessMgr.new(b, connect)
	}
}

func (b *Broker) sendMsgToClient(topic string, payload []byte, qos byte) {
	// plan, use topic from topic manager find clientID
	// use client ids find session
	// add message to session wait queue
	subscribers := b.topicMgr.findSubscribers(topic)
	if subscribers == nil {
		logger.Errorf("sendMsgToClient not find subscribers for topic <%s>", topic)
		return
	}
	for clientID := range subscribers {
		sess := b.sessMgr.get(clientID)
		if sess == nil {
			logger.Errorf("session for client <%s> is nil", clientID)
		} else {
			sess.publish(topic, payload, qos)
		}
	}
}

func (b *Broker) getClient(clientID string) *Client {
	b.RLock()
	defer b.RUnlock()

	if val, ok := b.clients[clientID]; ok {
		return val
	}
	return nil
}

func (b *Broker) close() {
	close(b.done)
	b.listener.Close()
	b.backend.close()
	b.sessMgr.close()

	b.Lock()
	defer b.Unlock()
	for _, v := range b.clients {
		v.close()
	}
	b.clients = nil

}
