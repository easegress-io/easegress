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
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/function/storage"
)

type (
	// Broker is MQTT server, will manage client, topic, session, etc.
	Broker struct {
		sync.RWMutex
		egName string
		name   string
		spec   *Spec

		listener net.Listener
		backend  BackendMQ
		clients  map[string]*Client
		auth     map[string]string
		tlsCfg   *tls.Config

		sessMgr   *SessionManager
		topicMgr  *TopicManager
		memberURL func(string, string) ([]string, error)

		// done is the channel for shutdowning this proxy.
		done chan struct{}
	}

	// HTTPJsonData is json data received from http endpoint used to send back to clients
	HTTPJsonData struct {
		Topic       string `json:"topic"`
		Qos         int    `json:"qos"`
		Payload     string `json:"payload"`
		Base64      bool   `json:"base64"`
		Distributed bool   `json:"distributed"`
	}
)

func newBroker(spec *Spec, store storage.Storage, memberURL func(string, string) ([]string, error)) *Broker {
	broker := &Broker{
		egName:    spec.EGName,
		name:      spec.Name,
		spec:      spec,
		backend:   newBackendMQ(spec),
		clients:   make(map[string]*Client),
		auth:      make(map[string]string),
		memberURL: memberURL,
		done:      make(chan struct{}),
	}
	broker.topicMgr = newTopicManager()
	broker.sessMgr = newSessionManager(broker, store)
	for _, a := range spec.Auth {
		broker.auth[a.Username] = a.B64Passwd
	}
	err := broker.setListener()
	if err != nil {
		logger.Errorf("mqtt.newBroker: broker set listener failed, err:%v", err)
		return nil
	}

	go broker.run()
	return broker
}

func (b *Broker) setListener() error {
	var l net.Listener
	var err error
	var cfg *tls.Config
	addr := fmt.Sprintf(":%d", b.spec.Port)
	if b.spec.UseTLS {
		cfg, err = b.spec.tlsConfig()
		if err != nil {
			return fmt.Errorf("invalid tls config for mqtt proxy, err:%v", err)
		}
		l, err = tls.Listen("tcp", addr, cfg)
		if err != nil {
			return fmt.Errorf("gen mqtt tls tcp listener failed, addr:%s, err:%v", addr, err)
		}
	} else {
		l, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("gen mqtt tcp listener failed, addr:%s, err:%v", addr, err)
		}
	}
	b.tlsCfg = cfg
	b.listener = l
	return err
}

func (b *Broker) run() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			select {
			case <-b.done:
				return
			default:
				logger.Errorf("mqtt.run: net listener accept failed, err:%s", err)
			}
		} else {
			go b.handleConn(conn)
		}
	}
}

func (b *Broker) checkClientAuth(connect *packets.ConnectPacket) bool {
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
		logger.Errorf("mqtt.handleConn: read connect packet failed, err:%s", err)
		return
	}
	connect, ok := packet.(*packets.ConnectPacket)
	if !ok {
		logger.Errorf("mqtt.handleConn: received %s that was not Connect", packet.String())
		return
	}

	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	connack.SessionPresent = connect.CleanSession
	connack.ReturnCode = connect.Validate()
	if connack.ReturnCode != packets.Accepted {
		err = connack.Write(conn)
		logger.Errorf("mqtt.handleConn: invalid connection %v, write connack failed, err:%s", connack.ReturnCode, err)
		return
	}

	if !b.checkClientAuth(connect) {
		connack.ReturnCode = packets.ErrRefusedNotAuthorised
		err = connack.Write(conn)
		logger.Errorf("mqtt.handleConn: invalid connection %v, connack back failed, err:%s", connack.ReturnCode, err)
		return
	}

	err = connack.Write(conn)
	if err != nil {
		logger.Errorf("mqtt.handleConn: send connack to client %s failed, err %s", connect.ClientIdentifier, err)
		return
	}

	client := newClient(connect, b, conn)
	cid := client.info.cid

	b.Lock()
	if oldClient, ok := b.clients[cid]; ok {
		go oldClient.close()
	}
	b.clients[client.info.cid] = client
	b.setSession(client, connect)
	b.Unlock()

	client.session.updateEGName(b.egName, b.name)
	topics, qoss, _ := client.session.allSubscribes()
	if len(topics) > 0 {
		err = b.topicMgr.subscribe(topics, qoss, client.info.cid)
		if err != nil {
			logger.Errorf("client <%v> use previous session topics <%v> to subscribe failed, err:%v", client.info.cid, topics, err)
		}
	}
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
		if prevSess != nil {
			prevSess.close()
		}
		client.session = b.sessMgr.newSessionFromConn(connect)
	}
}

func (b *Broker) requestTransfer(egName, name string, data HTTPJsonData) {
	urls, err := b.memberURL(egName, name)
	if err != nil {
		logger.Errorf("mqtt.requestTransfer: not find url for eg:%s, name:%s, err:%v", egName, name, err)
		return
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("mqtt.requestTransfer: json data marshal failed, err: %v", err)
		return
	}
	for _, url := range urls {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Errorf("mqtt.requestTransfer: make new request failed, err:%v", err)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Errorf("mqtt.requestTransfer: http client send msg failed, err:%v", err)
			return
		}
		defer resp.Body.Close()
	}
}

func (b *Broker) sendMsgToClient(topic string, payload []byte, qos byte) {
	subscribers, _ := b.topicMgr.findSubscribers(topic)
	if subscribers == nil {
		logger.Errorf("mqtt.sendMsgToClient: not find subscribers for topic %s", topic)
		return
	}

	for clientID, subQos := range subscribers {
		if subQos < qos {
			return
		}
		sess := b.sessMgr.get(clientID)
		if sess == nil {
			logger.Errorf("mqtt.sendMsgToClient: session for client %s is nil", clientID)
		} else {
			if sess.info.EGName == b.egName && sess.info.Name == b.name {
				sess.publish(topic, payload, qos)
			}
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

func (b *Broker) topicsPublishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	var data HTTPJsonData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid json data from request body"))
		return
	}
	if data.Qos < 0 || data.Qos > 2 {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("qos of MQTT is 0, 1, 2, and choose 1 for most cases"))
		return
	}
	var payload []byte
	if data.Base64 {
		payload, err = base64.StdEncoding.DecodeString(data.Payload)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("base64 set to true, but payload decode failed"))
			return
		}
	} else {
		payload = []byte(data.Payload)
	}

	if !data.Distributed {
		data.Distributed = true
		b.requestTransfer(b.egName, b.name, data)
	}
	go b.sendMsgToClient(data.Topic, payload, byte(data.Qos))
}

// const apiGroupName = "mqtt_proxy"

func (b *Broker) mqttAPIPrefix() string {
	return fmt.Sprintf(mqttAPIPrefix, b.name)
}

func (b *Broker) registerAPIs() {
	group := &api.Group{
		Group: b.name,
		Entries: []*api.Entry{
			{Path: b.mqttAPIPrefix(), Method: "POST", Handler: b.topicsPublishHandler},
		},
	}

	api.RegisterAPIs(group)
}

func (b *Broker) close() {
	close(b.done)
	b.listener.Close()
	b.backend.close()
	b.sessMgr.close()

	b.Lock()
	defer b.Unlock()
	for _, v := range b.clients {
		go v.close()
	}
	b.clients = nil
}
