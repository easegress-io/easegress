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

package connectcontrol

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/stretchr/testify/assert"
)

func newContext(cid string, topic string) context.MQTTContext {
	client := &context.MockMQTTClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	return context.NewMQTTContext(stdcontext.Background(), client, packet)
}

func TestConnectControl(t *testing.T) {
	meta := &pipeline.FilterMetaSpec{
		Name:     "connect-control-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
	}
	spec := &Spec{
		BannedClients: []string{"ban1", "ban2"},
		BannedTopics:  []string{"ban/sport/ball", "ban/sport/run"},
		EarlyStop:     true,
	}
	filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, spec, nil)
	cc := &ConnectControl{}
	cc.Init(filterSpec)

	tt := []struct {
		cid        string
		topic      string
		err        error
		disconnect bool
		earlyStop  bool
	}{
		{cid: "ban1", topic: "unban", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "ban2", topic: "unban", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban", topic: "ban/sport/ball", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban", topic: "ban/sport/run", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban", topic: "unban", err: nil, disconnect: false, earlyStop: false},
	}
	for _, test := range tt {
		ctx := newContext(test.cid, test.topic)
		res := cc.HandleMQTT(ctx)
		assert.Equal(t, res.Err, test.err)
		assert.Equal(t, ctx.Disconnect(), test.disconnect)
		assert.Equal(t, ctx.EarlyStop(), test.earlyStop)
	}

}

func TestHTTP(t *testing.T) {
	meta := &pipeline.FilterMetaSpec{
		Name:     "connect-control-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
	}
	spec := &Spec{
		BannedClients: []string{},
		BannedTopics:  []string{},
		EarlyStop:     true,
	}
	filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, spec, nil)
	cc := &ConnectControl{}
	cc.Init(filterSpec)

	tt := []struct {
		method     string
		reqData    *HTTPJsonData
		fn         http.HandlerFunc
		statusCode int
		resData    *HTTPJsonData
	}{
		{
			method:     http.MethodPost,
			reqData:    &HTTPJsonData{Clients: []string{"ban1"}, Topics: []string{"sport/ball"}},
			fn:         cc.handleBanClient,
			statusCode: 200,
			resData:    nil,
		},
		{
			method:     http.MethodGet,
			reqData:    nil,
			fn:         cc.handleInfo,
			statusCode: 200,
			resData:    &HTTPJsonData{Clients: []string{"ban1"}, Topics: []string{"sport/ball"}},
		},
		{
			method:     http.MethodPost,
			reqData:    &HTTPJsonData{Clients: []string{"ban1"}, Topics: []string{"sport/ball"}},
			fn:         cc.handleUnbanClient,
			statusCode: 200,
			resData:    nil,
		},
		{
			method:     http.MethodGet,
			reqData:    nil,
			fn:         cc.handleInfo,
			statusCode: 200,
			resData:    &HTTPJsonData{Clients: []string{}, Topics: []string{}},
		},
	}
	for _, test := range tt {
		var body io.Reader
		if test.reqData != nil {
			jsonData, err := json.Marshal(test.reqData)
			assert.Nil(t, err, "marshal json data failed")
			body = bytes.NewBuffer(jsonData)
		}
		req := httptest.NewRequest(test.method, "/fake", body)
		w := httptest.NewRecorder()
		test.fn(w, req)
		res := w.Result()
		assert.Equal(t, res.StatusCode, test.statusCode, "wrong code")
		if test.resData != nil {
			resData := &HTTPJsonData{}
			err := json.NewDecoder(res.Body).Decode(resData)
			assert.Nil(t, err, "decode json data failed")
			assert.Equal(t, resData, test.resData)
		}
		res.Body.Close()
	}
}
