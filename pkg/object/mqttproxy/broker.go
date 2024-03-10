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
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
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

		listener  net.Listener
		clients   map[string]*Client
		tlsCfg    *tls.Config
		pipelines map[PacketType]string
		muxMapper context.MuxMapper

		sessMgr           *SessionManager
		topicMgr          TopicManager
		sessionCacheMgr   SessionCacheManager
		connectionLimiter *Limiter
		memberURL         func(string, string) (map[string]string, error)

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

func getPipelineMap(spec *Spec) (map[PacketType]string, error) {
	ans := make(map[PacketType]string)

	// current only support route pipeline using packet type
	for _, rule := range spec.Rules {
		if _, ok := pipelinePacketTypes[rule.When.PacketType]; !ok {
			return nil, fmt.Errorf("pipeline packet type %v not found, only support %v", rule.When.PacketType, pipelinePacketTypes)
		}
		if _, ok := ans[rule.When.PacketType]; ok {
			return nil, fmt.Errorf("pipeline packet type %v show more than once", rule.When.PacketType)
		}
		ans[rule.When.PacketType] = rule.Pipeline
	}

	if _, ok := ans[Publish]; !ok {
		logger.Warnf("no pipeline for publish packet type to send MQTT message to backend")
	}
	if _, ok := ans[Connect]; !ok {
		logger.Warnf("no pipeline for connect packet type to check username and password of MQTT client")
	}
	return ans, nil
}

func newBroker(spec *Spec, store storage, muxMapper context.MuxMapper, memberURL func(string, string) (map[string]string, error)) *Broker {
	if spec.RetryInterval <= 0 {
		spec.RetryInterval = 30
	}

	broker := &Broker{
		egName:    spec.EGName,
		name:      spec.Name,
		spec:      spec,
		clients:   make(map[string]*Client),
		memberURL: memberURL,
		done:      make(chan struct{}),
		muxMapper: muxMapper,
	}
	pipelines, err := getPipelineMap(spec)
	if err != nil {
		panic(fmt.Sprintf("create pipeline map failed, %v", err))
	}
	broker.pipelines = pipelines

	err = broker.setListener()
	if err != nil {
		logger.SpanErrorf(nil, "mqtt broker set listener failed: %v", err)
		return nil
	}

	if spec.TopicCacheSize <= 0 {
		spec.TopicCacheSize = 100000
	}
	broker.topicMgr = newTopicManager(spec)
	broker.sessMgr = newSessionManager(broker, store)
	broker.connectionLimiter = newLimiter(spec.ConnectionLimit)
	go broker.run()

	if spec.BrokerMode {
		broker.sessionCacheMgr = newSessionCacheManager(spec, broker.topicMgr)
	}
	broker.connectWatcher()
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

func (b *Broker) connectWatcher() {
	var ch <-chan map[string]*string
	var cancelFunc func()
	var err error
	for {
		if b.closed() {
			return
		}

		ch, cancelFunc, err = b.sessMgr.store.watch(sessionStoreKey(""))
		if err == nil {
			break
		}
		logger.SpanErrorf(nil, "get watcher for session failed, %v", err)
		time.Sleep(10 * time.Second)
	}

	sessions, err := b.sessMgr.store.getPrefix(sessionStoreKey(""), false)
	if err != nil {
		logger.SpanErrorf(nil, "get all session prefix failed, %v", err)
	}

	if b.spec.BrokerMode {
		// make sessions a watcher event
		watcherEvent := make(map[string]*string)
		for k, v := range sessions {
			watcherEvent[k] = &v
		}
		b.processWatcherEvent(watcherEvent, true)
	}

	clients := []*Client{}
	b.Lock()
	for clientID, client := range b.clients {
		if _, ok := sessions[sessionStoreKey(clientID)]; !ok {
			clients = append(clients, client)
		}
	}
	b.Unlock()

	for _, c := range clients {
		c.close()
	}

	// start watch when finish connect and update
	go b.watch(ch, cancelFunc)
}

func (b *Broker) watch(ch <-chan map[string]*string, closeFunc func()) {
	defer closeFunc()
	for {
		select {
		case <-b.done:
			return
		case m := <-ch:
			if m == nil {
				go b.connectWatcher()
				return
			}
			b.processWatcherEvent(m, false)
		}
	}
}

func (b *Broker) processWatcherEvent(event map[string]*string, sync bool) {
	sessMap := make(map[string]*SessionInfo)
	for k, v := range event {
		clientID := strings.TrimPrefix(k, sessionStoreKey(""))
		if v != nil {
			var info *SessionInfo
			if b.spec.BrokerMode {
				session := &Session{}
				err := session.decode(*v)
				if err != nil {
					logger.Warnf("ignored decode session info %s failed: %s", *v, err)
					continue
				}
				info = session.info
				sessMap[clientID] = session.info
			}
			// the new session created, the scenario could indicate
			// that a device reconnect to a broker of the MQTT cluster,
			// so we kick out the previous session. Apparently, we
			// should disconnect session which reconnect same broker,
			// we should check the `egName` in new session (variable v indicated)
			// is a different value with current broker name. If it is a different
			// value, and the current broker contains the same session, it means that
			// we should disconnect session in the current broker, but we might attention
			// that we just disconnect session and don't clear global store session
			// information
			go b.handleNewSessionInCluster(clientID, v, info)

		} else {
			logger.Debugf("client %v recv delete watch %v", clientID, v)
			go b.deleteSession(clientID)

			if b.spec.BrokerMode {
				b.sessionCacheMgr.delete(clientID)
			}
		}
	}

	if b.spec.BrokerMode && len(sessMap) > 0 {
		if sync {
			b.sessionCacheMgr.sync(sessMap)
			return
		}
		b.sessionCacheMgr.update(sessMap)
	}
}

// processNewSession
func (b *Broker) handleNewSessionInCluster(clientID string, v *string, sessionInfo *SessionInfo) {
	c := b.getClient(clientID)
	if c == nil {
		// The current broker doesn't contain the new session, just ignore it.
		return
	}

	// The current broker contains the new session, so we need
	// to disconnect the current connection when the new session
	// was established to other brokers.
	var info *SessionInfo
	if sessionInfo != nil {
		info = sessionInfo
	} else {
		session := &Session{}
		err := session.decode(*v)
		if err != nil {
			logger.Warnf("ignored decorde session info %s failed: %s", *v, err)
			return
		}
		info = session.info
	}

	if info.EGName == b.egName {
		// The new session was established on the same broker,
		// just ignore it.
		return
	}

	// The new session was established on the different broker,
	// we should disconnect the connection in the current broker,
	// but we should not delete session information in the global
	// store (etcd).
	logger.Warnf("the client: %s was kicked out as the new session was eatablished on the broker: %s",
		c.ClientID(), info.EGName)
	c.kickOut()
	b.removeClient(clientID)

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

	authPipeline, ok := b.pipelines[Connect]
	if ok {
		pipe, ok := b.muxMapper.GetHandler(authPipeline)
		if !ok {
			logger.SpanErrorf(nil, "get pipeline %v failed", authPipeline)
			authFail = true
		} else {
			ctx := newContext(connect, client)
			pipe.Handle(ctx)
			res := ctx.GetResponse(context.DefaultNamespace).(*mqttprot.Response)
			if res.Disconnect() {
				logger.SpanErrorf(nil, "client %v not get connect permission from pipeline", connect.ClientIdentifier)
				authFail = true
			}
		}
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

	oldClient, err := func(clientID string) (*Client, error) {
		b.Lock()
		defer b.Unlock()
		if _, ok := b.clients[clientID]; ok {
			logger.SpanDebugf(nil, "client %v take over by new client with same name", clientID)
			oldClient := b.clients[clientID]
			b.clients[client.info.cid] = client
			return oldClient, nil
		}

		if b.spec.MaxAllowedConnection > 0 {
			if len(b.clients) >= b.spec.MaxAllowedConnection {
				logger.Errorf("client %v not get connect permission from rate limiter", connect.ClientIdentifier)
				connack.ReturnCode = packets.ErrRefusedServerUnavailable
				err = connack.Write(conn)
				if err != nil {
					logger.Errorf("connack back to client %s failed: %s", connect.ClientIdentifier, err)
				}
				return nil, nil
			}
		}
		b.clients[client.info.cid] = client
		return nil, nil
	}(client.info.cid)

	if err != nil {
		// Concurrent connection exceed maxium quotas, returned
		return
	}

	if oldClient != nil {
		// delete old client
		go oldClient.close()
	}

	b.setSession(client, connect)

	err = connack.Write(conn)
	if err != nil {
		logger.SpanErrorf(nil, "send connack to client %s failed: %s", connect.ClientIdentifier, err)
		// Don't clean client, dely to writeLoop or readLoop when error
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
	jsonData, err := codectool.MarshalJSON(data)
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
			continue
		}
		if b.spec.BrokerMode {
			egName := b.sessionCacheMgr.getEGName(clientID)
			if egName != b.egName {
				continue
			}
		}
		client := b.getClient(clientID)
		if client == nil {
			logger.SpanDebugf(span, "client %v not on broker %v in eg %v", clientID, b.name, b.egName)
		} else {
			client.session.publish(span, client, topic, payload, qos)
		}
	}
}

// splitSubscribers split subscribers to local and remote on broker mode and return
// client ids of local subscribers and eg names of remote subscribers
func (b *Broker) splitSubscribers(publish *packets.PublishPacket) ([]string, map[string]struct{}) {
	egNames := make(map[string]struct{})
	clients := []string{}

	subscribers, _ := b.topicMgr.findSubscribers(publish.TopicName)
	for clientID, subQos := range subscribers {
		if subQos < publish.Qos {
			continue
		}
		egName := b.sessionCacheMgr.getEGName(clientID)
		if egName != b.egName {
			egNames[egName] = struct{}{}
			continue
		}
		clients = append(clients, clientID)
	}
	return clients, egNames
}

func (b *Broker) sendMsgToLocalClient(span *model.SpanContext, publish *packets.PublishPacket, clients []string) {
	for _, clientID := range clients {
		client := b.getClient(clientID)
		if client == nil {
			logger.SpanDebugf(span, "client %v not on broker %v in eg %v", clientID, b.name, b.egName)
		} else {
			client.session.publish(span, client, publish.TopicName, publish.Payload, publish.Qos)
		}
	}
}

func (b *Broker) requestTransferToCertainInstances(span *model.SpanContext, publish *packets.PublishPacket, remoteEgs map[string]struct{}) {
	data := &HTTPJsonData{}
	data.init(publish)
	urls, err := b.memberURL(b.egName, b.name)
	if err != nil {
		logger.SpanErrorf(span, "eg %v find urls for other egs failed: %v", b.egName, err)
		return
	}
	jsonData, err := codectool.MarshalJSON(data)
	if err != nil {
		logger.SpanErrorf(span, "json data marshal failed: %v", err)
		return
	}
	for egName := range remoteEgs {
		url, ok := urls[egName]
		if !ok {
			logger.SpanErrorf(span, "eg %s not find url for eg %s", b.egName, egName)
			continue
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			logger.SpanErrorf(span, "make new request failed: %v", err)
			continue
		}
		err = b3.InjectHTTP(req, b3.WithSingleHeaderOnly())(*span)
		if err != nil {
			logger.SpanErrorf(span, "inject span failed: %v", err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.SpanErrorf(span, "http client send msg failed:%v", err)
		} else {
			resp.Body.Close()
		}
	}

}

func (b *Broker) processBrokerModePublish(clientID string, publish *packets.PublishPacket) {
	if !b.spec.BrokerMode {
		return
	}
	localClients, remoteEgs := b.splitSubscribers(publish)
	if len(localClients) == 0 && len(remoteEgs) == 0 {
		return
	}

	span := generateNewSpanContext(clientID, publish.TopicName)
	if len(localClients) > 0 {
		b.sendMsgToLocalClient(span, publish, localClients)
	}
	if len(remoteEgs) > 0 {
		b.requestTransferToCertainInstances(span, publish, remoteEgs)
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
	err := codectool.DecodeJSON(r.Body, &data)
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

	jsonData, err := codectool.MarshalJSON(res)
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
	err := codectool.DecodeJSON(r.Body, &data)
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
	b.sessMgr.close()
	b.topicMgr.close()
	if b.spec.BrokerMode {
		b.sessionCacheMgr.close()
	}

	b.Lock()
	defer b.Unlock()
	for _, v := range b.clients {
		go v.closeAndDelSession()
	}
	b.clients = nil
}

func newContext(packet packets.ControlPacket, client mqttprot.Client) *context.Context {
	ctx := context.New(tracing.NoopSpan)
	req := mqttprot.NewRequest(packet, client)
	ctx.SetRequest(context.DefaultNamespace, req)
	resp := mqttprot.NewResponse()
	ctx.SetResponse(context.DefaultNamespace, resp)
	return ctx
}

func generateNewSpanContext(clientID string, topic string) *model.SpanContext {
	span := &model.SpanContext{}
	span.TraceID.High = rand.Uint64()
	span.TraceID.Low = rand.Uint64()
	span.ID = model.ID(rand.Uint64())
	logger.SpanDebugf(span, "create new span for client %v and topic %v", clientID, topic)
	return span
}

func (d *HTTPJsonData) init(packet *packets.PublishPacket) {
	d.Topic = packet.TopicName
	d.QoS = int(packet.Qos)
	d.Payload = base64.StdEncoding.EncodeToString(packet.Payload)
	d.Base64 = true
	d.Distributed = true
}
