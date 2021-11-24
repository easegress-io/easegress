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
	stdcontext "context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/propagation/b3"
)

type (
	// Broker is MQTT server, will manage client, topic, session, etc.
	Broker struct {
		sync.RWMutex
		egName string
		name   string
		spec   *Spec

		listener   net.Listener
		backend    backendMQ
		clients    map[string]*Client
		sha256Auth map[string]string
		tlsCfg     *tls.Config
		pipeline   string

		sessMgr           *SessionManager
		topicMgr          *TopicManager
		connectionLimiter *Limiter
		memberURL         func(string, string) ([]string, error)

		// done is the channel for shutdowning this proxy.
		done chan struct{}
	}

	// HTTPJsonData is json data received from http endpoint used to send back to clients
	HTTPJsonData struct {
		Topic       string `json:"topic"`
		QoS         int    `json:"qos"`
		Payload     string `json:"payload"`
		Base64      bool   `json:"base64"`
		Distributed bool   `json:"distributed"`
	}
)

func sha256Sum(data []byte) string {
	sha256Bytes := sha256.Sum256(data)
	return hex.EncodeToString(sha256Bytes[:])
}

func newBroker(spec *Spec, store storage, memberURL func(string, string) ([]string, error)) *Broker {
	broker := &Broker{
		egName:     spec.EGName,
		name:       spec.Name,
		spec:       spec,
		pipeline:   spec.Pipeline,
		backend:    newBackendMQ(spec),
		clients:    make(map[string]*Client),
		sha256Auth: make(map[string]string),
		memberURL:  memberURL,
		done:       make(chan struct{}),
	}

	if !spec.AuthByPipeline {
		for _, a := range spec.Auth {
			passwd, err := base64.StdEncoding.DecodeString(a.PassBase64)
			if err != nil {
				spanErrorf(nil, "auth with name %v, base64 password %v decode failed: %v", a.UserName, a.PassBase64, err)
				return nil
			}
			broker.sha256Auth[a.UserName] = sha256Sum(passwd)
		}
		if len(broker.sha256Auth) == 0 {
			spanErrorf(nil, "empty valid auth for mqtt proxy")
			return nil
		}
	}

	err := broker.setListener()
	if err != nil {
		spanErrorf(nil, "mqtt broker set listener failed: %v", err)
		return nil
	}

	if spec.TopicCacheSize <= 0 {
		spec.TopicCacheSize = 100000
	}
	broker.topicMgr = newTopicManager(spec.TopicCacheSize)
	broker.sessMgr = newSessionManager(broker, store)
	broker.connectionLimiter = newLimiter(spec.ConnectionLimit)
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
			return fmt.Errorf("invalid tls config for mqtt proxy: %v", err)
		}
		l, err = tls.Listen("tcp", addr, cfg)
		if err != nil {
			return fmt.Errorf("gen mqtt tls tcp listener with addr %v and cfg %v failed: %v", addr, cfg, err)
		}
	} else {
		l, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("gen mqtt tcp listener with addr %s failed: %v", addr, err)
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
			}
		} else {
			go b.handleConn(conn)
		}
	}
}

func (b *Broker) checkClientAuth(connect *packets.ConnectPacket) bool {
	cid := connect.ClientIdentifier
	name := connect.Username
	sha256Passwd := sha256Sum(connect.Password)
	if authpasswd, ok := b.sha256Auth[name]; ok {
		return (cid != "") && (sha256Passwd == authpasswd)
	}
	return false
}

func (b *Broker) checkConnectPermission(connect *packets.ConnectPacket) bool {
	size := connect.RemainingLength + 8
	permitted := b.connectionLimiter.acquirePermission(size)
	if !permitted {
		return permitted
	}
	if b.spec.MaxAllowedConnection > 0 {
		b.Lock()
		connNum := len(b.clients)
		b.Unlock()
		if connNum >= b.spec.MaxAllowedConnection {
			return false
		}
	}
	return true
}

func (b *Broker) handleConn(conn net.Conn) {
	defer conn.Close()
	packet, err := packets.ReadPacket(conn)
	if err != nil {
		spanErrorf(nil, "read connect packet failed: %s", err)
		return
	}
	connect, ok := packet.(*packets.ConnectPacket)
	if !ok {
		spanErrorf(nil, "first packet received %s that was not Connect", packet.String())
		return
	}
	spanDebugf(nil, "connection from client %s", connect.ClientIdentifier)

	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	connack.SessionPresent = connect.CleanSession
	connack.ReturnCode = connect.Validate()
	if connack.ReturnCode != packets.Accepted {
		err = connack.Write(conn)
		spanErrorf(nil, "invalid connection %v, write connack failed: %s", connack.ReturnCode, err)
		return
	}

	if !b.checkConnectPermission(connect) {
		connack.ReturnCode = packets.ErrRefusedServerUnavailable
		err = connack.Write(conn)
		if err != nil {
			spanErrorf(nil, "connack back to client %s failed: %s", connect.ClientIdentifier, err)
		}
		return
	}

	client := newClient(connect, b, conn, b.spec.ClientPublishLimit)

	authFail := false
	if b.spec.AuthByPipeline {
		pipe, err := pipeline.GetPipeline(b.pipeline, context.MQTT)
		if err != nil {
			spanErrorf(nil, "get pipeline %v failed, %v", b.pipeline, err)
			authFail = true
		} else {
			ctx := context.NewMQTTContext(stdcontext.Background(), b.backend, client, connect)
			pipe.HandleMQTT(ctx)
			if ctx.Disconnect() {
				authFail = true
			}
		}
	} else if !b.checkClientAuth(connect) {
		authFail = true
	}
	if authFail {
		connack.ReturnCode = packets.ErrRefusedNotAuthorised
		err = connack.Write(conn)
		if err != nil {
			spanErrorf(nil, "connack back to client %s failed: %s", connect.ClientIdentifier, err)
		}
		spanErrorf(nil, "invalid connection %v, client %s auth failed", connack.ReturnCode, connect.ClientIdentifier)
		return
	}

	err = connack.Write(conn)
	if err != nil {
		spanErrorf(nil, "send connack to client %s failed: %s", connect.ClientIdentifier, err)
		return
	}

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
			spanErrorf(nil, "client %v use previous session topics %v to subscribe failed: %v", client.info.cid, topics, err)
		}
	}
	go client.writeLoop()
	client.readLoop()
}

func (b *Broker) setSession(client *Client, connect *packets.ConnectPacket) {
	// when clean session is false, previous session exist and previous session not clean session,
	// then we use previous session, otherwise use new session
	prevSess := b.sessMgr.get(connect.ClientIdentifier)
	if !connect.CleanSession && (prevSess != nil) && !prevSess.cleanSession() {
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
		spanErrorf(nil, "find urls for other egs failed:%v", err)
		return
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		spanErrorf(nil, "json data marshal failed: %v", err)
		return
	}
	for _, url := range urls {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			spanErrorf(nil, "make new request failed: %v", err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			spanErrorf(nil, "http client send msg failed:%v", err)
		} else {
			resp.Body.Close()
		}
	}
	spanDebugf(nil, "http transfer data %v to %v", data, urls)
}

func (b *Broker) sendMsgToClient(span *model.SpanContext, topic string, payload []byte, qos byte) {
	subscribers, _ := b.topicMgr.findSubscribers(topic)
	spanDebugf(span, "send topic %v to client %v", topic, subscribers)
	if subscribers == nil {
		spanErrorf(span, "not find subscribers for topic %s", topic)
		return
	}

	for clientID, subQoS := range subscribers {
		if subQoS < qos {
			return
		}
		client := b.getClient(clientID)
		if client == nil {
			spanDebugf(span, "client %v not on broker %v", clientID, b.name)
		} else {
			client.session.publish(span, topic, payload, qos)
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

func (b *Broker) removeClient(clientID string) {
	b.Lock()
	if val, ok := b.clients[clientID]; ok {
		if val.disconnected() {
			delete(b.clients, clientID)
		}
	}
	b.Unlock()
}

func (b *Broker) topicsPublishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	var data HTTPJsonData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid json data from request body"))
		return
	}
	if data.QoS < int(QoS0) || data.QoS > int(QoS2) {
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

	spanDebugf(nil, "http endpoint received json data: %v", data)
	if !data.Distributed {
		data.Distributed = true
		b.requestTransfer(b.egName, b.name, data)
	}
	span, _ := b3.ExtractHTTP(r)()
	go b.sendMsgToClient(span, data.Topic, payload, byte(data.QoS))
}

func (b *Broker) mqttAPIPrefix() string {
	return fmt.Sprintf(mqttAPIPrefix, b.name)
}

func (b *Broker) registerAPIs() {
	group := &api.Group{
		Group: b.name,
		Entries: []*api.Entry{
			{Path: b.mqttAPIPrefix(), Method: http.MethodPost, Handler: b.topicsPublishHandler},
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
		go v.closeAndDelSession()
	}
	b.clients = nil
}
