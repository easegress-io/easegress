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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
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
		done      chan struct{}
		closeFlag int32
	}

	// HTTPJsonData is json data received from http endpoint used to send back to clients
	HTTPJsonData struct {
		Topic       string `json:"topic"`
		QoS         int    `json:"qos"`
		Payload     string `json:"payload"`
		Base64      bool   `json:"base64"`
		Distributed bool   `json:"distributed"`
	}

	// HTTPSessions is json data used for session related operations, like get all sessions and delete some sessions
	HTTPSessions struct {
		Sessions []*HTTPSession `json:"sessions"`
	}

	// HTTPSession is json data used for session related operations, like get all sessions and delete some sessions
	HTTPSession struct {
		SessionID string `json:"sessionID"`
		Topic     string `json:"topic"`
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
	if broker.backend == nil {
		panic(fmt.Sprintf("mqtt broker %v connect backend failed", broker.name))
	}

	if !spec.AuthByPipeline {
		for _, a := range spec.Auth {
			passwd, err := base64.StdEncoding.DecodeString(a.PassBase64)
			if err != nil {
				logger.SpanErrorf(nil, "auth with name %v, base64 password %v decode failed: %v", a.UserName, a.PassBase64, err)
				return nil
			}
			broker.sha256Auth[a.UserName] = sha256Sum(passwd)
		}
		if len(broker.sha256Auth) == 0 {
			logger.SpanErrorf(nil, "empty valid auth for mqtt proxy")
			return nil
		}
	}

	err := broker.setListener()
	if err != nil {
		logger.SpanErrorf(nil, "mqtt broker set listener failed: %v", err)
		return nil
	}

	if spec.TopicCacheSize <= 0 {
		spec.TopicCacheSize = 100000
	}
	broker.topicMgr = newTopicManager(spec.TopicCacheSize)
	broker.sessMgr = newSessionManager(broker, store)
	broker.connectionLimiter = newLimiter(spec.ConnectionLimit)
	go broker.run()
	ch, closeFunc, err := broker.sessMgr.store.watchDelete(sessionStoreKey(""))
	if err != nil {
		logger.SpanErrorf(nil, "get watcher for session failed, %v", err)
	}
	if ch != nil {
		go broker.watchDelete(ch, closeFunc)
	}
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

func (b *Broker) reconnectWatcher() {
	if b.closed() {
		return
	}

	ch, cancelFunc, err := b.sessMgr.store.watchDelete(sessionStoreKey(""))
	if err != nil {
		logger.SpanErrorf(nil, "get watcher for session failed, %v", err)
		time.Sleep(10 * time.Second)
		go b.reconnectWatcher()
		return
	}
	go b.watchDelete(ch, cancelFunc)

	// check event during reconnect
	sessions, err := b.sessMgr.store.getPrefix(sessionStoreKey(""), true)
	if err != nil {
		logger.SpanErrorf(nil, "get all session prefix failed, %v", err)
	}

	clients := []*Client{}
	b.Lock()
	for sessionID, client := range b.clients {
		if _, ok := sessions[sessionID]; !ok {
			clients = append(clients, client)
		}
	}
	b.Unlock()

	for _, c := range clients {
		c.close()
	}
}

func (b *Broker) watchDelete(ch <-chan map[string]*string, closeFunc func()) {
	defer closeFunc()
	for {
		select {
		case <-b.done:
			return
		case m := <-ch:
			if m == nil {
				go b.reconnectWatcher()
				return
			}
			for k, v := range m {
				if v != nil {
					continue
				}
				clientID := strings.TrimPrefix(k, sessionStoreKey(""))
				logger.SpanDebugf(nil, "client %v recv delete watch %v", clientID, v)
				go b.deleteSession(clientID)
			}
		}
	}
}

func (b *Broker) deleteSession(clientID string) {
	b.Lock()
	defer b.Unlock()
	if c, ok := b.clients[clientID]; ok {
		if !c.disconnected() {
			logger.SpanDebugf(nil, "broker watch and delete client %v", c.info.cid)
			c.close()
		}
	}
	delete(b.clients, clientID)
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
	// check here to do early stop for connection. Later we will check it again to make sure
	// not exceed MaxAllowedConnection
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

func (b *Broker) connectionValidation(connect *packets.ConnectPacket, conn net.Conn) (*Client, *packets.ConnackPacket, bool) {
	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	connack.SessionPresent = connect.CleanSession
	connack.ReturnCode = connect.Validate()
	if connack.ReturnCode != packets.Accepted {
		err := connack.Write(conn)
		logger.SpanErrorf(nil, "invalid connection %v, write connack failed: %s", connack.ReturnCode, err)
		return nil, nil, false
	}
	// check rate limiter and max allowed connection
	if !b.checkConnectPermission(connect) {
		logger.SpanDebugf(nil, "client %v not get connect permission from rate limiter", connect.ClientIdentifier)
		connack.ReturnCode = packets.ErrRefusedServerUnavailable
		err := connack.Write(conn)
		if err != nil {
			logger.SpanErrorf(nil, "connack back to client %s failed: %s", connect.ClientIdentifier, err)
		}
		return nil, nil, false
	}

	client := newClient(connect, b, conn, b.spec.ClientPublishLimit)
	// check auth
	authFail := false
	if b.spec.AuthByPipeline {
		pipe, err := pipeline.GetPipeline(b.pipeline, context.MQTT)
		if err != nil {
			logger.SpanErrorf(nil, "get pipeline %v failed, %v", b.pipeline, err)
			authFail = true
		} else {
			ctx := context.NewMQTTContext(stdcontext.Background(), b.backend, client, connect)
			pipe.HandleMQTT(ctx)
			if ctx.Disconnect() {
				logger.SpanErrorf(nil, "client %v not get connect permission from pipeline", connect.ClientIdentifier)
				authFail = true
			}
		}
	} else if !b.checkClientAuth(connect) {
		authFail = true
	}
	if authFail {
		connack.ReturnCode = packets.ErrRefusedNotAuthorised
		err := connack.Write(conn)
		if err != nil {
			logger.SpanErrorf(nil, "connack back to client %s failed: %s", connect.ClientIdentifier, err)
		}
		logger.SpanErrorf(nil, "invalid connection %v, client %s auth failed", connack.ReturnCode, connect.ClientIdentifier)
		return nil, nil, false
	}
	return client, connack, true
}

func (b *Broker) handleConn(conn net.Conn) {
	defer conn.Close()
	packet, err := packets.ReadPacket(conn)
	if err != nil {
		logger.SpanErrorf(nil, "read connect packet failed: %s", err)
		return
	}
	connect, ok := packet.(*packets.ConnectPacket)
	if !ok {
		logger.SpanErrorf(nil, "first packet received %s that was not Connect", packet.String())
		return
	}
	logger.SpanDebugf(nil, "connection from client %s", connect.ClientIdentifier)

	client, connack, valid := b.connectionValidation(connect, conn)
	if !valid {
		return
	}
	cid := client.info.cid

	b.Lock()
	if oldClient, ok := b.clients[cid]; ok {
		logger.SpanDebugf(nil, "client %v take over by new client with same name", oldClient.info.cid)
		go oldClient.close()

	} else if b.spec.MaxAllowedConnection > 0 {
		if len(b.clients) >= b.spec.MaxAllowedConnection {
			logger.SpanDebugf(nil, "client %v not get connect permission from rate limiter", connect.ClientIdentifier)
			connack.ReturnCode = packets.ErrRefusedServerUnavailable
			err = connack.Write(conn)
			if err != nil {
				logger.SpanErrorf(nil, "connack back to client %s failed: %s", connect.ClientIdentifier, err)
			}
			b.Unlock()
			return
		}
	}
	b.clients[client.info.cid] = client
	b.setSession(client, connect)
	b.Unlock()

	err = connack.Write(conn)
	if err != nil {
		logger.SpanErrorf(nil, "send connack to client %s failed: %s", connect.ClientIdentifier, err)
		return
	}

	client.session.updateEGName(b.egName, b.name)
	topics, qoss, _ := client.session.allSubscribes()
	if len(topics) > 0 {
		err = b.topicMgr.subscribe(topics, qoss, client.info.cid)
		if err != nil {
			logger.SpanErrorf(nil, "client %v use previous session topics %v to subscribe failed: %v", client.info.cid, topics, err)
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

func (b *Broker) requestTransfer(span *model.SpanContext, egName, name string, data HTTPJsonData, header http.Header) {
	urls, err := b.memberURL(egName, name)
	if err != nil {
		logger.SpanErrorf(span, "eg %v find urls for other egs failed:%v", b.egName, err)
		return
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.SpanErrorf(span, "json data marshal failed: %v", err)
		return
	}
	for _, url := range urls {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
		req.Header = header.Clone()
		if err != nil {
			logger.SpanErrorf(span, "make new request failed: %v", err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.SpanErrorf(span, "http client send msg failed:%v", err)
		} else {
			resp.Body.Close()
		}
	}
	logger.SpanDebugf(span, "eg %v http transfer data %v to %v", b.egName, data, urls)
}

func (b *Broker) sendMsgToClient(span *model.SpanContext, topic string, payload []byte, qos byte) {
	subscribers, _ := b.topicMgr.findSubscribers(topic)
	logger.SpanDebugf(span, "eg %v send topic %v to client %v", b.egName, topic, subscribers)
	if subscribers == nil {
		logger.SpanErrorf(span, "eg %v not find subscribers for topic %s", b.egName, topic)
		return
	}

	for clientID, subQoS := range subscribers {
		if subQoS < qos {
			return
		}
		client := b.getClient(clientID)
		if client == nil {
			logger.SpanDebugf(span, "client %v not on broker %v in eg %v", clientID, b.name, b.egName)
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

func (b *Broker) httpTopicsPublishHandler(w http.ResponseWriter, r *http.Request) {
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

	span, _ := b3.ExtractHTTP(r)()
	logger.SpanDebugf(span, "http endpoint received json data: %v", data)
	if !data.Distributed {
		data.Distributed = true
		headers := r.Header.Clone()
		b.requestTransfer(span, b.egName, b.name, data, headers)
	}
	go b.sendMsgToClient(span, data.Topic, payload, byte(data.QoS))
}

func (b *Broker) mqttAPIPrefix(path string) string {
	return fmt.Sprintf(path, b.name)
}

func (b *Broker) httpGetAllSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose Get request but got %s", r.Method))
		return
	}
	span, _ := b3.ExtractHTTP(r)()
	logger.SpanDebugf(span, "http endpoint receive request to get all session")

	query := r.URL.Query()
	page := 0
	pageSize := 0
	var topic string
	var err error
	if len(query) != 0 {
		pageStr := query.Get("page")
		pageSizeStr := query.Get("page_size")
		topic = query.Get("q")
		if len(pageStr) == 0 || len(pageSizeStr) == 0 {
			api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("if use query, please provide both page and page_size"))
			return
		}
		page, err = strconv.Atoi(pageStr)
		if err != nil || page <= 0 {
			api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("page in query should be number and >0"))
			return
		}
		pageSize, err = strconv.Atoi(pageSizeStr)
		if err != nil || pageSize <= 0 {
			api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("page_size in query should be number and >0"))
			return
		}
	}

	allSession, err := b.sessMgr.store.getPrefix(sessionStoreKey(""), false)
	logger.SpanDebugf(span, "httpGetAllSessionHandler current total %v sessions, query %v, topic %v", len(allSession), []int{page, pageSize}, topic)
	if err != nil {
		logger.SpanErrorf(span, "get all sessions with prefix %v failed, %v", sessionStoreKey(""), err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("get all sessions failed, %v", err))
		return
	}

	res := b.queryAllSessions(allSession, len(query) != 0, page, pageSize, topic)

	jsonData, err := json.Marshal(res)
	if err != nil {
		logger.SpanErrorf(span, "all session data json marshal failed, %v", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("all sessions json marshal failed, %v", err))
		return
	}
	_, err = w.Write(jsonData)
	if err != nil {
		logger.SpanErrorf(span, "write json data to http response writer failed, %v", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("write json data failed"))
	}
}

func (b *Broker) queryAllSessions(allSession map[string]string, query bool, page, pageSize int, topic string) *HTTPSessions {
	res := &HTTPSessions{}
	if !query {
		for k := range allSession {
			httpSession := &HTTPSession{
				SessionID: strings.TrimPrefix(k, sessionStoreKey("")),
			}
			res.Sessions = append(res.Sessions, httpSession)
		}
		return res
	}

	index := 0
	start := page*pageSize - pageSize
	end := page * pageSize
	for _, v := range allSession {
		if index >= start && index < end {
			session := &Session{}
			session.info = &SessionInfo{}
			session.decode(v)
			for k := range session.info.Topics {
				if strings.Contains(k, topic) {
					httpSession := &HTTPSession{
						SessionID: session.info.ClientID,
						Topic:     k,
					}
					res.Sessions = append(res.Sessions, httpSession)
					break
				}
			}
		}
		if index > end {
			break
		}
		index++
	}
	return res
}

func (b *Broker) httpDeleteSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("suppose POST request but got %s", r.Method))
		return
	}
	var data HTTPSessions
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid json data from request body"))
		return
	}

	span, _ := b3.ExtractHTTP(r)()
	logger.SpanDebugf(span, "http endpoint received delete session data: %v", data)
	for _, s := range data.Sessions {
		err := b.sessMgr.store.delete(sessionStoreKey(s.SessionID))
		if err != nil {
			logger.SpanErrorf(span, "delete session %v failed, %v", s, err)
		}
	}
}

func (b *Broker) currentClients() map[string]struct{} {
	ans := make(map[string]struct{})
	b.Lock()
	for k := range b.clients {
		ans[k] = struct{}{}
	}
	b.Unlock()
	return ans
}

func (b *Broker) registerAPIs() {
	group := &api.Group{
		Group: b.name,
		Entries: []*api.Entry{
			{Path: b.mqttAPIPrefix(mqttAPITopicPublishPrefix), Method: http.MethodPost, Handler: b.httpTopicsPublishHandler},
			{Path: b.mqttAPIPrefix(mqttAPISessionQueryPrefix), Method: http.MethodGet, Handler: b.httpGetAllSessionHandler},
			{Path: b.mqttAPIPrefix(mqttAPISessionDeletePrefix), Method: http.MethodDelete, Handler: b.httpDeleteSessionHandler},
		},
	}

	api.RegisterAPIs(group)
}

func (b *Broker) setClose() {
	atomic.StoreInt32(&b.closeFlag, 1)
}

func (b *Broker) closed() bool {
	flag := atomic.LoadInt32(&b.closeFlag)
	return flag == 1
}

func (b *Broker) close() {
	b.setClose()
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
