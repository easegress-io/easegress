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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"testing"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
	"gopkg.in/yaml.v2"
)

func (t *testMQ) get() *packets.PublishPacket {
	p := <-t.ch
	return p
}

func init() {
	logger.InitNop()
}

func getMQTTClient(t *testing.T, clientID, userName, password string, cleanSession bool) paho.Client {
	opts := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID(clientID).SetUsername(userName).SetPassword(password).SetCleanSession(cleanSession)
	c := paho.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Errorf("basic connect error for client <%s> with <%s>", clientID, token.Error())
	}
	return c
}

func getMQTTSubscribeHandler(ch chan CheckMsg) paho.MessageHandler {
	handler := func(client paho.Client, message paho.Message) {
		msg := CheckMsg{
			topic:   message.Topic(),
			payload: string(message.Payload()),
			qos:     int(message.Qos()),
		}
		ch <- msg
	}
	return handler
}

func getBroker(name, userName, passBase64 string, port uint16) *Broker {
	spec := &Spec{
		Name:        name,
		EGName:      name,
		Port:        port,
		BackendType: testMQType,
		Auth: []Auth{
			{UserName: userName, PassBase64: passBase64},
		},
	}
	store := newStorage(nil)
	broker := newBroker(spec, store, func(s, ss string) ([]string, error) {
		m := map[string]string{
			"test":  "http://localhost:8888/mqtt",
			"test1": "http://localhost:8889/mqtt",
		}
		urls := []string{}
		for k, v := range m {
			if k != s {
				urls = append(urls, v)
			}
		}
		return urls, nil
	})
	return broker
}

func TestConnection(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	c1 := getMQTTClient(t, "test", "test", "test", true)
	c1.Disconnect(200)

	o2 := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID("test").SetUsername("fakeuser").SetPassword("fakepasswd")
	c2 := paho.NewClient(o2)
	if token := c2.Connect(); token.Wait() && token.Error() == nil {
		t.Errorf("non auth user should fail%s", token.Error())
	}
	c2.Disconnect(200)

	broker.close()
}

func TestPublish(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	client := getMQTTClient(t, "test", "test", "test", true)

	for i := 0; i < 5; i++ {
		topic := "go-mqtt/sample"
		text := fmt.Sprintf("qos0 msg #%d!", i)
		token := client.Publish(topic, 0, false, text)
		token.Wait()
		p := broker.backend.(*testMQ).get()
		if p.TopicName != topic || string(p.Payload) != text {
			t.Errorf("get wrong publish")
		}
	}

	for i := 0; i < 5; i++ {
		topic := "go-mqtt/sample"
		text := fmt.Sprintf("qos1 msg #%d!", i)
		token := client.Publish(topic, 1, false, text)
		token.Wait()
		if token.Error() != nil {
			t.Errorf("should support qos1")
		}
		p := broker.backend.(*testMQ).get()
		if p.TopicName != topic || string(p.Payload) != text {
			t.Errorf("get wrong publish")
		}
	}
	client.Disconnect(200)

	broker.close()
}

func TestSubUnsub(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	client := getMQTTClient(t, "test", "test", "test", true)
	if token := client.Subscribe("go-mqtt/qos0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	if token := client.Subscribe("go-mqtt/qos1", 1, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	if token := client.Unsubscribe("go-mqtt/sample"); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	client.Disconnect(200)
	broker.close()
}

func TestCleanSession(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	// client that set cleanSession
	cid := "cleanSessionClient"
	client := getMQTTClient(t, cid, "test", "test", true)
	if token := client.Subscribe("test/cleanSession/0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	client.Disconnect(200)

	c := broker.getClient(cid)
	if c != nil {
		c.closeAndDelSession()
	}
	subscribers, err := broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; ok {
		t.Errorf("topicMgr is not cleaned when client %v close connection", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err == nil {
		t.Errorf("not clean DB when cleanSession is set, potential resource wasted")
	}

	// client that not set cleanSession, getMQTTClient last parameter set to false
	cid = "notCleanSessionClient"
	client = getMQTTClient(t, cid, "test", "test", false)
	if token := client.Subscribe("test/cleanSession/0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	client.Disconnect(200)

	c = broker.getClient(cid)
	if c != nil {
		c.closeAndDelSession()
	}
	subscribers, err = broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; ok {
		t.Errorf("topicMgr is not cleaned when client %v close connection", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err != nil {
		t.Errorf("clean DB when cleanSession is not set")
	}

	// check not clean session work when client come.
	client = getMQTTClient(t, cid, "test", "test", false)
	// publish topic to make sure client read loop start
	token := client.Publish("topic", 1, false, "text")
	token.Wait()
	subscribers, err = broker.topicMgr.findSubscribers("test/cleanSession/0")
	if err != nil {
		t.Errorf("findSubscribers for topic test/cleanSession/0 failed, %v", err)
	}
	if _, ok := subscribers[cid]; !ok {
		t.Errorf("topicMgr should contain topic test/cleanSession/0 when client reconnect, but got %v", subscribers)
	}
	_, err = broker.sessMgr.store.get(sessionStoreKey(cid))
	if err != nil {
		t.Errorf("clean DB when cleanSession is not set")
	}

	broker.close()
}

func TestMultiClientPublish(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	var wg sync.WaitGroup

	clientNum := 30
	msgNum := 50
	for i := 0; i < clientNum; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cid := fmt.Sprintf("test_%d", i)
			client := getMQTTClient(t, cid, "test", "test", true)
			for j := 0; j < msgNum; j++ {
				topic := cid
				text := fmt.Sprintf("%d", j)
				token := client.Publish(topic, 1, false, text)
				token.Wait()
				if token.Error() != nil {
					t.Errorf("client <%d> publish failed, <%s>", i, token.Error())
				}
			}
			client.Disconnect(200)
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		producer := broker.backend.(*testMQ)
		ans := make(map[string]int)
		for i := 0; i < clientNum*msgNum; i++ {
			p := producer.get()
			num, _ := strconv.Atoi(string(p.Payload))
			if val, ok := ans[p.TopicName]; ok {
				if num != val+1 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			} else {
				if num != 0 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			}
		}
	}()

	wg.Wait()
	broker.close()
}

func TestSession(t *testing.T) {
	name := "admin"
	passwd := "passwd"
	b64passwd := base64.StdEncoding.EncodeToString([]byte(passwd))
	broker := getBroker("test", name, b64passwd, 1883)

	client := getMQTTClient(t, "test", name, passwd, true)

	// sub go-mqtt/qos0 and check
	if token := client.Subscribe("go-mqtt/qos0", 0, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos0 error %s", token.Error())
	}
	subs, qoss, _ := broker.sessMgr.get("test").allSubscribes()
	if subs[0] != "go-mqtt/qos0" || qoss[0] != 0 {
		t.Errorf("session error")
	}

	// sub go-mqtt/qos1 and check
	if token := client.Subscribe("go-mqtt/qos1", 1, nil); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	subs, qoss, _ = broker.sessMgr.get("test").allSubscribes()
	if len(subs) != 2 || len(qoss) != 2 {
		t.Errorf("session error")
	}

	// unsub go-mqtt/qos0 and check
	if token := client.Unsubscribe("go-mqtt/qos0"); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}
	subs, qoss, _ = broker.sessMgr.get("test").allSubscribes()
	if subs[0] != "go-mqtt/qos1" || qoss[0] != 1 {
		t.Errorf("session error")
	}

	sessMgr := broker.sessMgr
	sess := Session{
		info: &SessionInfo{
			EGName:    "eg",
			Name:      "mqtt",
			ClientID:  "checkStoreGet",
			Topics:    map[string]int{"a": 1},
			CleanFlag: false,
		},
	}
	str, _ := sess.encode()
	sessMgr.store.put(sessionStoreKey("checkStoreGet"), str)
	newSess := sessMgr.get("checkStoreGet")
	if !reflect.DeepEqual(newSess.info, sess.info) {
		t.Errorf("sessMgr get wrong sess %v, %v", sess.info, newSess.info)
	}
	client.Disconnect(200)
	broker.close()
}

func TestSpec(t *testing.T) {
	yamlStr := `
    port: 1883
    backendType: Kafka
    auth:
      - userName: test
        passBase64: dGVzdA==
      - userName: admin
        passBase64: YWRtaW4=
    topicMapper:
      matchIndex: 0
      route: 
        - name: g2s
          matchExpr: g2s
        - name: d2s
          matchExpr: d2s
      policies:
        - name: d2s
          topicIndex: 1
          route: 
            - topic: to_cloud
              exprs: ["bar", "foo"]
            - topic: to_raw
              exprs: [".*"] 
          headers: 
            0: d2s
            1: type
            2: device
        - name: g2s
          topicIndex: 4
          route:
            - topic: to_cloud
              exprs: ["bar", "foo"]
            - topic: to_raw
              exprs: [".*"]
          headers: 
            0: g2s
            1: gateway
            2: info
            3: device
            4: type
    kafkaBroker:
      backend: ["123.123.123.123:9092", "234.234.234.234:9092"]
    useTLS: true
    certificate:
      - name: cert1
        cert: balabala
        key: keyForbalabala
      - name: cert2
        cert: foo
        key: bar
`
	got := Spec{}
	err := yaml.Unmarshal([]byte(yamlStr), &got)
	if err != nil {
		t.Errorf("yaml unmarshal failed, err:%v", err)
	}

	want := Spec{
		Port:        1883,
		BackendType: "Kafka",
		Auth: []Auth{
			{"test", "dGVzdA=="},
			{"admin", "YWRtaW4="},
		},
		TopicMapper: &TopicMapper{
			MatchIndex: 0,
			Route: []*PolicyRe{
				{"g2s", "g2s"},
				{"d2s", "d2s"},
			},
			Policies: []*Policy{
				{
					Name:       "d2s",
					TopicIndex: 1,
					Route: []TopicRe{
						{"to_cloud", []string{"bar", "foo"}},
						{"to_raw", []string{".*"}},
					},
					Headers: map[int]string{0: "d2s", 1: "type", 2: "device"},
				},
				{
					Name:       "g2s",
					TopicIndex: 4,
					Route: []TopicRe{
						{"to_cloud", []string{"bar", "foo"}},
						{"to_raw", []string{".*"}},
					},
					Headers: map[int]string{0: "g2s", 1: "gateway", 2: "info", 3: "device", 4: "type"},
				},
			},
		},
		Kafka: &KafkaSpec{
			Backend: []string{"123.123.123.123:9092", "234.234.234.234:9092"},
		},
		UseTLS: true,
		Certificate: []Certificate{
			{"cert1", "balabala", "keyForbalabala"},
			{"cert2", "foo", "bar"},
		},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("got:<%#v>\n want:<%#v>\n", got, want)
	}
}

func TestTopicMapper(t *testing.T) {
	// /d2s/{tenant}/{device_type}/{things_id}/log/eventName
	// /d2s/{tenant}/{device_type}/{things_id}/status
	// /d2s/{tenant}/{device_type}/{things_id}/event
	// /d2s/{tenant}/{device_type}/{things_id}/raw
	// /g2s/{gwTenantId}/{gwInfoModelId}/{gwThingsId}/d2s/{tenantId}/{infoModelId}/{thingsId}/data
	topicMapper := TopicMapper{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"g2s", "g2s"},
			{"d2s", "d2s"},
		},
		Policies: []*Policy{
			{
				Name:       "d2s",
				TopicIndex: 4,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					0: "d2s",
					1: "tenant",
					2: "device_type",
					3: "things_id",
					4: "event",
					5: "eventName",
				},
			},
			{
				Name:       "g2s",
				TopicIndex: 8,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event", "data"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					0: "g2s",
					1: "gwTenantId",
					2: "gwInfoModelId",
					3: "gwThingsId",
					4: "d2s",
					5: "tenantId",
					6: "infoModelId",
					7: "thingsId",
					8: "event",
					9: "eventName",
				},
			},
		},
	}
	mapFunc := getTopicMapFunc(&topicMapper)
	tests := []struct {
		mqttTopic string
		topic     string
		headers   map[string]string
	}{
		{
			mqttTopic: "/d2s/abc/phone/123/log/error",
			topic:     "to_cloud",
			headers:   map[string]string{"d2s": "d2s", "tenant": "abc", "device_type": "phone", "things_id": "123", "event": "log", "eventName": "error"}},
		{
			mqttTopic: "/d2s/xyz/tv/234/status/shutdown",
			topic:     "to_cloud",
			headers:   map[string]string{"d2s": "d2s", "tenant": "xyz", "device_type": "tv", "things_id": "234", "event": "status", "eventName": "shutdown"}},
		{
			mqttTopic: "/d2s/opq/car/345/raw",
			topic:     "to_raw",
			headers:   map[string]string{"d2s": "d2s", "tenant": "opq", "device_type": "car", "things_id": "345", "event": "raw"}},
		{
			mqttTopic: "/g2s/gwTenantId/gwInfoModelId/gwThingsId/d2s/tenantId/infoModelId/thingsId/data",
			topic:     "to_cloud",
			headers: map[string]string{"g2s": "g2s", "gwTenantId": "gwTenantId", "gwInfoModelId": "gwInfoModelId", "gwThingsId": "gwThingsId",
				"d2s": "d2s", "tenantId": "tenantId", "infoModelId": "infoModelId", "thingsId": "thingsId", "event": "data"}},
		{
			mqttTopic: "/g2s/gw123/gwInfo234/gwID345/d2s/456/654/123/raw",
			topic:     "to_raw",
			headers: map[string]string{"g2s": "g2s", "gwTenantId": "gw123", "gwInfoModelId": "gwInfo234", "gwThingsId": "gwID345",
				"d2s": "d2s", "tenantId": "456", "infoModelId": "654", "thingsId": "123", "event": "raw"}},
	}
	for _, tt := range tests {
		topic, headers, err := mapFunc(tt.mqttTopic)
		if err != nil || topic != tt.topic || !reflect.DeepEqual(tt.headers, headers) {
			if !reflect.DeepEqual(tt.headers, headers) {
				fmt.Printf("headers not equal, <%v>, <%v>\n", tt.headers, headers)
			}
			t.Errorf("unexpected result from topic map function <%+v>, <%+v>, want:<%+v>, got:<%+v>\n\n\n\n", tt.mqttTopic, topic, tt.headers, headers)
		}
	}

	_, _, err := mapFunc("/not-work")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	mapFunc = getTopicMapFunc(nil)
	if mapFunc != nil {
		t.Errorf("nil mapFunc if TopicMapper is nil")
	}

	fakeMapper := TopicMapper{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"fake", "]["},
		},
	}
	getTopicMapFunc(&fakeMapper)
	fakeMapper = TopicMapper{
		MatchIndex: 0,
		Route: []*PolicyRe{
			{"demo", "demo"},
		},
		Policies: []*Policy{
			{
				Name:       "demo",
				TopicIndex: 1,
				Route: []TopicRe{
					{"topic", []string{"]["}},
				},
				Headers: map[int]string{1: "topic"},
			},
		},
	}
	getTopicMapFunc(&fakeMapper)
}

func TestTopicMapper2(t *testing.T) {
	// nothing/d2s/{tenant}/{device_type}/{things_id}/log/eventName
	// nothing/d2s/{tenant}/{device_type}/{things_id}/status
	// nothing/d2s/{tenant}/{device_type}/{things_id}/event
	// nothing/d2s/{tenant}/{device_type}/{things_id}/raw
	// nothing/g2s/{gwTenantId}/{gwInfoModelId}/{gwThingsId}/d2s/{tenantId}/{infoModelId}/{thingsId}/data
	topicMapper := TopicMapper{
		MatchIndex: 1,
		Route: []*PolicyRe{
			{"g2s", "g2s"},
			{"d2s", "d2s"},
		},
		Policies: []*Policy{
			{
				Name:       "d2s",
				TopicIndex: 5,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					1: "d2s",
					2: "tenant",
					3: "device_type",
					4: "things_id",
					5: "event",
				},
			},
			{
				Name:       "g2s",
				TopicIndex: 9,
				Route: []TopicRe{
					{"to_cloud", []string{"log", "status", "event", "data"}},
					{"to_raw", []string{"raw"}},
				},
				Headers: map[int]string{
					1: "g2s",
					2: "gwTenantId",
					3: "gwInfoModelId",
					4: "gwThingsId",
					5: "d2s",
					6: "tenantId",
					7: "infoModelId",
					8: "thingsId",
					9: "event",
				},
			},
		},
	}
	mapFunc := getTopicMapFunc(&topicMapper)

	_, _, err := mapFunc("/level0")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	_, _, err = mapFunc("nothing/g2s")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
	_, _, err = mapFunc("nothing/d2s/tenant/device_type/things_id/notexist")
	if err == nil {
		t.Errorf("map func should return err for not match topic")
	}
}

type CheckMsg struct {
	topic   string
	payload string
	qos     int
}

func TestSendMsgBack(t *testing.T) {
	clientNum := 10
	msgNum := 100
	subscribeCh := make(chan CheckMsg, 100)
	var wg sync.WaitGroup

	// make broker
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	// subscribe handler
	handler := getMQTTSubscribeHandler(subscribeCh)
	// make client and subscribe different topic
	clients := []paho.Client{}
	for i := 0; i < clientNum; i++ {
		cid := fmt.Sprintf("test_%d", i)
		c := getMQTTClient(t, cid, "test", "test", true)
		if token := c.Subscribe(cid, 1, handler); token.Wait() && token.Error() != nil {
			t.Errorf("subscribe qos1 error %s", token.Error())
		}
		clients = append(clients, c)
	}
	fmt.Printf("TestSendMsgBack: finished get clients\n")

	// publish msg and send msg back
	for i := 0; i < clientNum; i++ {
		wg.Add(2)
		go func(c paho.Client) {
			defer wg.Done()
			r := c.OptionsReader()
			for j := 0; j < msgNum; j++ {
				topic := r.ClientID()
				text := fmt.Sprintf("%d", j)
				token := c.Publish(topic, 1, false, text)
				token.Wait()
				if token.Error() != nil {
					t.Errorf("client <%d> publish failed, <%s>", i, token.Error())
				}
			}
			fmt.Printf("client %s finished publish\n", r.ClientID())
		}(clients[i])

		go func(c paho.Client) {
			defer wg.Done()
			r := c.OptionsReader()
			for j := 0; j < msgNum; j++ {
				topic := r.ClientID()
				text := fmt.Sprintf("sub %d", j)
				broker.sendMsgToClient(topic, []byte(text), QoS1)
			}
		}(clients[i])
	}
	fmt.Printf("TestSendMsgBack: finished publish and send msg back\n")

	wg.Add(2)
	// receive msg
	go func() {
		defer func() {
			fmt.Printf("all msg received!\n")
			wg.Done()
		}()
		producer := broker.backend.(*testMQ)
		ans := make(map[string]int)
		for i := 0; i < clientNum*msgNum; i++ {
			p := producer.get()
			num, _ := strconv.Atoi(string(p.Payload))
			if val, ok := ans[p.TopicName]; ok {
				if num != val+1 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			} else {
				if num != 0 {
					t.Errorf("received publish not in order")
				}
				ans[p.TopicName] = num
			}
		}
	}()

	// check get enough msg, there may be resend
	checkAns := func(ans map[string]map[string]struct{}) bool {
		if len(ans) < clientNum {
			return false
		}
		for _, v := range ans {
			if len(v) < msgNum {
				return false
			}
		}
		return true
	}

	// receive subscribes
	go func() {
		defer func() {
			fmt.Printf("all subscribes received!\n")
			wg.Done()
		}()
		ans := make(map[string]map[string]struct{})

		for {
			if checkAns(ans) {
				break
			}
			msg := <-subscribeCh
			if _, ok := ans[msg.topic]; ok {
				ans[msg.topic][msg.payload] = struct{}{}
			} else {
				ans[msg.topic] = make(map[string]struct{})
				ans[msg.topic][msg.payload] = struct{}{}
			}
		}

		for _, v := range ans {
			if len(v) != msgNum {
				t.Errorf("not receive right number of msg")
			}
		}
	}()

	fmt.Printf("TestSendMsgBack: wait received all msg and subscribes\n")
	// close
	wg.Wait()
	fmt.Printf("TestSendMsgBack: received all msg and subscribes\n")

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-subscribeCh:
			}
		}
	}()

	broker.close()
	for _, c := range clients {
		go c.Disconnect(200)
	}
	close(done)
}

func TestYamlEncodeDecode(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	// old session
	s := &Session{
		broker: broker,
		info: &SessionInfo{
			Topics:    map[string]int{"abc": 1, "def": 2},
			ClientID:  "test",
			CleanFlag: true,
		},
	}
	str, err := s.encode()
	if err != nil {
		t.Errorf("%s", err.Error())
	}

	// new session
	newS := Session{
		info: &SessionInfo{},
	}
	if err := newS.decode(str); err != nil {
		t.Errorf("yaml Decode error")
	}

	if !reflect.DeepEqual(newS.info.Topics, s.info.Topics) || newS.info.ClientID != s.info.ClientID || newS.info.CleanFlag != s.info.CleanFlag {
		t.Errorf("yaml encode decode error")
	}
	broker.close()
}

type testServer struct {
	mux *http.ServeMux
	srv http.Server
}

func newServer(addr string) *testServer {
	mux := http.NewServeMux()
	srv := http.Server{Addr: addr, Handler: mux}
	ts := &testServer{
		mux: mux,
		srv: srv,
	}
	return ts
}

func (ts *testServer) addHandlerFunc(pattern string, f http.HandlerFunc) {
	ts.mux.HandleFunc(pattern, f)
}

func (ts *testServer) start() {
	go ts.srv.ListenAndServe()
}

func (ts *testServer) shutdown() {
	ts.srv.Shutdown(context.Background())
}

func topicsPublish(t *testing.T, data HTTPJsonData) int {
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Errorf("json marshal error")
	}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8888/mqtt", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Errorf("request mqtt error")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error, %s", err.Error())
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestHTTPRequest(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	srv := newServer(":8888")
	srv.addHandlerFunc("/mqtt", broker.topicsPublishHandler)
	srv.start()

	// GET request should fail
	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8888/mqtt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("client do req error %s", err)
	}
	if resp.StatusCode == http.StatusOK {
		t.Errorf("http GET should fail, only POST permitted")
	}
	resp.Body.Close()

	// non json data should fail
	req, err = http.NewRequest(http.MethodPost, "http://localhost:8888/mqtt", bytes.NewBuffer([]byte("non json")))
	if err != nil {
		t.Errorf("request mqtt error")
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error")
	}
	if resp.StatusCode == http.StatusOK {
		t.Errorf("http post should put json data")
	}
	resp.Body.Close()

	// err qos should fail
	data := HTTPJsonData{
		Topic:   "topic",
		QoS:     10,
		Payload: "data",
		Base64:  false,
	}
	statusCode := topicsPublish(t, data)
	if statusCode == http.StatusOK {
		t.Errorf("qos must be 0 or 1 or 2")
	}

	// base64 flag
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     10,
		Payload: "data",
		Base64:  true,
	}
	statusCode = topicsPublish(t, data)
	if statusCode == http.StatusOK {
		t.Errorf("not use base64 but set base64 true")
	}

	// success
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     1,
		Payload: "data",
		Base64:  false,
	}
	statusCode = topicsPublish(t, data)
	if statusCode != http.StatusOK {
		t.Errorf("request should success")
	}
	data = HTTPJsonData{
		Topic:   "topic",
		QoS:     1,
		Payload: base64.StdEncoding.EncodeToString([]byte("data")),
		Base64:  true,
	}
	statusCode = topicsPublish(t, data)
	if statusCode != http.StatusOK {
		t.Errorf("request should success")
	}

	broker.close()
	srv.shutdown()
}

func TestHTTPPublish(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	srv := newServer(":8888")
	srv.addHandlerFunc("/mqtt", broker.topicsPublishHandler)
	srv.start()

	subscribeCh := make(chan CheckMsg)
	handler := getMQTTSubscribeHandler(subscribeCh)
	client := getMQTTClient(t, "test", "test", "test", true)
	if token := client.Subscribe("test", 1, handler); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}

	numMsg := 200
	go func() {
		for i := 0; i < numMsg; i++ {
			data := HTTPJsonData{
				Topic:   "test",
				QoS:     1,
				Payload: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", i))),
				Base64:  true,
			}
			go func(data HTTPJsonData) {
				code := topicsPublish(t, data)
				if code != http.StatusOK {
					t.Errorf("wrong status code return")
				}
			}(data)
		}

	}()

	ans := map[string]struct{}{}
	for len(ans) < numMsg {
		msg := <-subscribeCh
		if msg.topic != "test" {
			t.Errorf("wrong topic received")
		}
		ans[msg.payload] = struct{}{}
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-subscribeCh:
			}
		}
	}()

	client.Disconnect(200)
	broker.close()
	srv.shutdown()
	close(done)
}

func TestHTTPTransfer(t *testing.T) {
	passBase64 := base64.StdEncoding.EncodeToString([]byte("test"))
	broker0 := getBroker("test", "test", passBase64, 1883)
	srv0 := newServer(":8888")
	srv0.addHandlerFunc("/mqtt", broker0.topicsPublishHandler)
	srv0.start()

	spec := &Spec{
		Name:        "test1",
		EGName:      "test1",
		Port:        1884,
		BackendType: testMQType,
		Auth: []Auth{
			{UserName: "test", PassBase64: passBase64},
		},
	}
	store := broker0.sessMgr.store
	broker1 := newBroker(spec, store, func(s, ss string) ([]string, error) {
		m := map[string]string{
			"test":  "http://localhost:8888/mqtt",
			"test1": "http://localhost:8889/mqtt",
		}
		urls := []string{}
		for k, v := range m {
			if k != s {
				urls = append(urls, v)
			}
		}
		return urls, nil
	})
	srv1 := newServer(":8889")
	srv1.addHandlerFunc("/mqtt", broker1.topicsPublishHandler)
	srv1.start()

	// auto connect to broker0, not broker1
	ch := make(chan CheckMsg)
	handler := getMQTTSubscribeHandler(ch)
	mqttClient := getMQTTClient(t, "client", "test", "test", true)
	if token := mqttClient.Subscribe("client", 1, handler); token.Wait() && token.Error() != nil {
		t.Errorf("subscribe qos1 error %s", token.Error())
	}

	// set data to broker1
	data := HTTPJsonData{
		Topic:   "client",
		QoS:     1,
		Payload: "data",
		Base64:  false,
	}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:8889/mqtt", bytes.NewBuffer(jsonData))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("mqtt client do error")
	}
	defer resp.Body.Close()

	// get data from client who connect to broker0
	msg := <-ch
	if msg.topic != "client" || msg.payload != "data" {
		t.Errorf("wrong msg")
	}
	go func() {
		msg = <-ch
		t.Errorf("receive data again, may transfer data for multi times")
	}()

	broker0.close()
	broker1.close()
	srv0.shutdown()
	srv1.shutdown()

	go func() {
		for {
			<-ch
		}
	}()
	mqttClient.Disconnect(200)
}

func TestSplitTopic(t *testing.T) {
	tests := []struct {
		topic  string
		levels []string
		ans    bool
	}{
		{"#", []string{"#"}, true},
		{"#/sport", nil, false},
		{"sport#", nil, false},
		{"/#/sport", nil, false},
		{"#+", nil, false},
		{"/+/sport/+/player1/+/score", []string{"", "+", "sport", "+", "player1", "+", "score"}, true},
		{"/+/++/+finance", nil, false},
		{"/sport/ball/player1/score", []string{"", "sport", "ball", "player1", "score"}, true},
		{"finance/+/bank/#", []string{"finance", "+", "bank", "#"}, true},
		{"/sport/ball/player1/score/+/wimbledon/#", []string{"", "sport", "ball", "player1", "score", "+", "wimbledon", "#"}, true},
		{"/sport/ball/player2/score/+/game2/#", []string{"", "sport", "ball", "player2", "score", "+", "game2", "#"}, true},
		{"/", []string{"", ""}, true},
		{"finance/", []string{"finance", ""}, true},
		{"//", []string{"", "", ""}, true},
		{"/finance/bank", []string{"", "finance", "bank"}, true},
		{"/finance/bank/", []string{"", "finance", "bank", ""}, true},
		{"sport/tennis/player1/#", []string{"sport", "tennis", "player1", "#"}, true},
		{"sport/tennis/player1", []string{"sport", "tennis", "player1"}, true},
		{"sport/tennis/player1/ranking", []string{"sport", "tennis", "player1", "ranking"}, true},
		{"sport/tennis/player1/score/wimbledon", []string{"sport", "tennis", "player1", "score", "wimbledon"}, true},
		{"sport/#", []string{"sport", "#"}, true},
		{"sport/tennis/#", []string{"sport", "tennis", "#"}, true},
		{"sport/tennis#", nil, false},
		{"sport/tennis/#/ranking", nil, false},
		{"sport/tennis/+", []string{"sport", "tennis", "+"}, true},
		{"sport/tennis/player1", []string{"sport", "tennis", "player1"}, true},
		{"+", []string{"+"}, true},
		{"+/tennis/#", []string{"+", "tennis", "#"}, true},
		{"sport+", nil, false},
		{"sport/+/player1", []string{"sport", "+", "player1"}, true},
		{"/finance", []string{"", "finance"}, true},
	}
	for _, tt := range tests {
		levels, ans := splitTopic(tt.topic)
		if !reflect.DeepEqual(levels, tt.levels) {
			t.Errorf("topic:<%s> levels got:%v, want:%v", tt.topic, levels, tt.levels)
		}
		if ans != tt.ans {
			t.Errorf("topic:<%s>, got:%v, want:%v", tt.topic, ans, tt.ans)
		}
	}
}

func TestWildCard(t *testing.T) {
	mgr := newTopicManager(10000)
	mgr.subscribe([]string{"a/+", "b/d"}, []byte{0, 1}, "A")
	mgr.subscribe([]string{"+/+"}, []byte{1}, "B")
	mgr.subscribe([]string{"+/fin"}, []byte{1}, "C")
	mgr.subscribe([]string{"a/fin"}, []byte{1}, "D")
	mgr.subscribe([]string{"#"}, []byte{2}, "E")
	mgr.subscribe([]string{"a/#"}, []byte{2}, "F")

	subscribers, _ := mgr.findSubscribers("a/fin")
	want := map[string]byte{"A": 0, "B": 1, "C": 1, "D": 1, "E": 2, "F": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	subscribers, _ = mgr.findSubscribers("b/d")
	want = map[string]byte{"A": 1, "B": 1, "E": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	subscribers, _ = mgr.findSubscribers("b/d/f")
	want = map[string]byte{"E": 2}
	if !reflect.DeepEqual(subscribers, want) {
		t.Errorf("test wild card want:%v, got:%v\n", want, subscribers)
	}

	err := mgr.subscribe([]string{"++", "##"}, []byte{0, 1}, "A")
	if err == nil {
		t.Errorf("subscribe invalid topic should return error")
	}
	err = mgr.unsubscribe([]string{"++", "##", "/a/b/++"}, "A")
	if err == nil {
		t.Errorf("unsubscribe invalid topic should return error")
	}
	_, err = mgr.findSubscribers("##")
	if err == nil {
		t.Errorf("find subscribers for invalid topic should return error")
	}

	mgr = newTopicManager(100000)
	mgr.subscribe([]string{"a/b/c/d/e"}, []byte{0}, "A")
	mgr.subscribe([]string{"a/b/c/f/g"}, []byte{0}, "A")
	mgr.subscribe([]string{"m/x/v/f/g"}, []byte{0}, "B")
	mgr.subscribe([]string{"m/x"}, []byte{0}, "C")

	mgr.unsubscribe([]string{"a/b/c/d/e"}, "A")
	mgr.unsubscribe([]string{"a/b/c/f/g"}, "A")
	mgr.unsubscribe([]string{"m/x/v/f/g"}, "B")
	mgr.unsubscribe([]string{"m/x"}, "C")
	if len(mgr.root.clients) != 0 || len(mgr.root.nodes) != 0 {
		t.Errorf("topic manager not clear memory when topics are unsubscribes")
	}

	mgr = newTopicManager(1000000)
	checkSubscriptions := func(subTopic string, recvTopic []string, notRecvTopic []string) {
		mgr.subscribe([]string{subTopic}, []byte{0}, "tmpClient")
		for _, topic := range recvTopic {
			clients, err := mgr.findSubscribers(topic)
			if err != nil {
				t.Errorf("findSubscribers for topic failed, %v", err)
			}
			if val, ok := clients["tmpClient"]; !ok || (val != 0) {
				t.Errorf("subscribe topic %v but not recv topic %v", subTopic, topic)
			}
		}
		for _, topic := range notRecvTopic {
			clients, err := mgr.findSubscribers(topic)
			if err != nil {
				t.Errorf("findSubscribers for topic failed, %v", err)
			}
			if _, ok := clients["tmpClient"]; ok {
				t.Errorf("subscribe topic %v but recv topic %v", subTopic, topic)
			}
		}
		mgr.unsubscribe([]string{subTopic}, "tmpClient")
	}
	checkSubscriptions("sport/tennis/player1/#", []string{"sport/tennis/player1", "sport/tennis/player1/ranking", "sport/tennis/player1/score/wimbledon"}, []string{})
	checkSubscriptions("sport/#", []string{"sport"}, []string{})

	checkSubscriptions("sport/tennis/+", []string{"sport/tennis/player1", "sport/tennis/player2"}, []string{"sport/tennis/player1/ranking"})
	checkSubscriptions("sport/+", []string{"sport/"}, []string{"sport"})
	checkSubscriptions("+/+", []string{"/finance"}, []string{})
	checkSubscriptions("/+", []string{"/finance"}, []string{})
	checkSubscriptions("+", []string{}, []string{"/finance"})
}

const certPem = `
-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`
const keyPem = `
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----
`

func TestTLSConfig(t *testing.T) {
	spec := Spec{}
	_, err := spec.tlsConfig()
	if err == nil {
		t.Errorf("no certificate, should return error")
	}

	spec = Spec{
		Certificate: []Certificate{
			{"fake", "fakeCert", "fakeKey"},
		},
	}
	_, err = spec.tlsConfig()
	if err == nil {
		t.Errorf("no vallid certificate, should return error")
	}

	spec = Spec{
		Certificate: []Certificate{
			{"demo", string(certPem), string(keyPem)},
		},
	}
	_, err = spec.tlsConfig()
	if err != nil {
		t.Errorf("should return nil for correct cert and key pair err:<%v>", err)
	}
}

func TestSessMgr(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)
	sessMgr := broker.sessMgr
	sess := &Session{
		storeCh: sessMgr.storeCh,
		info: &SessionInfo{
			EGName:    "testEg",
			Name:      "mqttProxy",
			Topics:    map[string]int{"a": 1, "b": 0},
			ClientID:  "testClient",
			CleanFlag: true,
		},
	}
	sessStr, err := sess.encode()
	if err != nil {
		t.Errorf("session encode failed, err:%v", err)
	}
	newSess := sessMgr.newSessionFromYaml(&sessStr)
	if !reflect.DeepEqual(sess.info, newSess.info) {
		t.Errorf("sessMgr produce wrong session")
	}
	broker.close()
}

func TestClient(t *testing.T) {
	svcConn, clientConn := net.Pipe()
	go func() {
		io.ReadAll(svcConn)
	}()
	connect := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connect.WillFlag = true
	connect.WillQos = 1
	connect.WillTopic = "will"
	connect.WillMessage = []byte("i am gone")

	client := newClient(connect, nil, clientConn)
	will := client.info.will
	if (will.Qos != connect.WillQos) || (will.TopicName != connect.WillTopic) || string(will.Payload) != string(connect.WillMessage) {
		t.Error("produce wrong will msg")
	}

	err := client.processPacket(connect)
	if err == nil {
		t.Errorf("double connect should return error")
	}
	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	err = client.processPacket(connack)
	if err == nil {
		t.Errorf("client should not send connack")
	}

	pubrec := packets.NewControlPacket(packets.Pubrec).(*packets.PubrecPacket)
	err = client.processPacket(pubrec)
	if err == nil {
		t.Errorf("qos2 not support now")
	}

	suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
	err = client.processPacket(suback)
	if err == nil {
		t.Errorf("server not subscribe")
	}
	unsuback := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
	err = client.processPacket(unsuback)
	if err == nil {
		t.Errorf("server not subscribe")
	}

	pingreq := packets.NewControlPacket(packets.Pingreq).(*packets.PingreqPacket)
	err = client.processPacket(pingreq)
	if err != nil {
		t.Errorf("ping should success")
	}
	pingresp := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
	err = client.processPacket(pingresp)
	if err == nil {
		t.Errorf("broker not ping")
	}
}

func TestBrokerListen(t *testing.T) {
	// invalid tls
	spec := &Spec{
		Port:        1883,
		BackendType: testMQType,
		Auth: []Auth{
			{UserName: "test", PassBase64: "test"},
		},
		UseTLS: true,
		Certificate: []Certificate{
			{"fake", "abc", "abc"},
		},
	}
	store := newStorage(nil)
	broker := newBroker(spec, store, func(s, ss string) ([]string, error) {
		return nil, nil
	})
	if broker != nil {
		t.Errorf("invalid tls config should return nil broker")
	}

	// valid tls
	spec = &Spec{
		Port:        1883,
		BackendType: testMQType,
		Auth: []Auth{
			{UserName: "test", PassBase64: base64.StdEncoding.EncodeToString([]byte("test"))},
		},
		UseTLS: true,
		Certificate: []Certificate{
			{"demo", certPem, keyPem},
		},
	}
	broker = newBroker(spec, store, func(s, ss string) ([]string, error) {
		return nil, nil
	})

	if broker == nil {
		t.Errorf("valid tls config should not return nil broker")
	}

	broker1 := newBroker(spec, store, func(s, ss string) ([]string, error) {
		return nil, nil
	})
	if broker1 != nil {
		t.Errorf("not valid port should return nil broker")
	}

	// not valid port should return nil
	spec = &Spec{
		Port:        1883,
		BackendType: testMQType,
		Auth: []Auth{
			{UserName: "test", PassBase64: "test"},
		},
	}
	broker2 := newBroker(spec, store, func(s, ss string) ([]string, error) {
		return nil, nil
	})
	if broker2 != nil {
		t.Errorf("not valid port should return nil broker")
	}
	broker.mqttAPIPrefix()
	broker.registerAPIs()
	broker.close()
}

func TestBrokerHandleConn(t *testing.T) {
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)

	// broker handleConn return if error happen
	svcConn, clientConn := net.Pipe()
	go clientConn.Write([]byte("fake data for paho.packets.ReadPacket to return error, to make that happends, this fake data should be long enough."))
	broker.handleConn(svcConn)

	// not use connect to connect
	svcConn, clientConn = net.Pipe()
	subscribe := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	go subscribe.Write(clientConn)
	broker.handleConn(svcConn)

	// use in valid connect to connect
	svcConn, clientConn = net.Pipe()
	connect := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connect.ReservedBit = 1
	go connect.Write(clientConn)
	go func() {
		io.ReadAll(clientConn)
	}()
	broker.handleConn(svcConn)
	broker.close()
}

func TestMQTTProxy(t *testing.T) {
	mp := MQTTProxy{}
	mp.Status()
	b64passwd := base64.StdEncoding.EncodeToString([]byte("test"))
	broker := getBroker("test", "test", b64passwd, 1883)
	mp.broker = broker
	mp.Close()
}
