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

func defaultFilterSpec() *pipeline.FilterSpec {
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
	return filterSpec
}

func TestConnectControl(t *testing.T) {
	assert := assert.New(t)

	cc := &ConnectControl{}
	assert.Equal(cc.Kind(), Kind, "wrong kind")
	assert.Equal(cc.DefaultSpec(), &Spec{}, "wrong spec")
	assert.NotEqual(len(cc.Description()), 0, "description for ConnectControl is empty")
	assert.Nil(cc.Results(), "if update result, please update this case")
	assert.Nil(cc.Status(), "if update status, please update this case")
	checkProtocol := func() (err error) {
		defer func() {
			if errMsg := recover(); errMsg != nil {
				err = errors.New(errMsg.(string))
				return
			}
		}()
		meta := &pipeline.FilterMetaSpec{
			Protocol: context.HTTP,
		}
		filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, nil, nil)
		cc.Init(filterSpec)
		return
	}
	err := checkProtocol()
	assert.NotNil(err, "if ConnectControl supports more protocol, please update this case")

	filterSpec := defaultFilterSpec()
	cc.Init(filterSpec)
	newCc := &ConnectControl{}
	newCc.Inherit(filterSpec, cc)
	defer newCc.Close()
}

func TestHandleMQTT(t *testing.T) {
	assert := assert.New(t)

	filterSpec := defaultFilterSpec()
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
		assert.Equal(res.Err, test.err)
		assert.Equal(ctx.Disconnect(), test.disconnect)
		assert.Equal(ctx.EarlyStop(), test.earlyStop)
	}

	packet := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	ctx := context.NewMQTTContext(stdcontext.Background(), nil, packet)
	res := cc.HandleMQTT(ctx)
	assert.Equal(res.Err, errors.New(resultWrongPacket), "should return error for wrong result")
}

func TestHTTP(t *testing.T) {
	assert := assert.New(t)

	meta := &pipeline.FilterMetaSpec{
		Name:     "connect-control-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
	}
	spec := &Spec{}
	filterSpec := pipeline.MockFilterSpec(nil, nil, "", meta, spec, nil)
	cc := &ConnectControl{}
	cc.Init(filterSpec)
	apis := cc.APIs()
	assert.Equal(len(apis), 3, "if apis is updated, please update this case")

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
			assert.Nil(err, "marshal json data failed")
			body = bytes.NewBuffer(jsonData)
		}
		req := httptest.NewRequest(test.method, "/fake", body)
		w := httptest.NewRecorder()
		test.fn(w, req)
		res := w.Result()
		assert.Equal(res.StatusCode, test.statusCode, "wrong code")
		if test.resData != nil {
			resData := &HTTPJsonData{}
			err := json.NewDecoder(res.Body).Decode(resData)
			assert.Nil(err, "decode json data failed")
			assert.Equal(resData, test.resData)
		}
		res.Body.Close()
	}

	checkBadReq := func(r *http.Request, fn http.HandlerFunc) {
		w := httptest.NewRecorder()
		fn(w, r)
		res := w.Result()
		assert.Equal(res.StatusCode, http.StatusBadRequest)
		res.Body.Close()
	}

	// wrong method
	checkBadReq(httptest.NewRequest(http.MethodGet, "/fake", nil), cc.handleBanClient)
	checkBadReq(httptest.NewRequest(http.MethodGet, "/fake", nil), cc.handleUnbanClient)
	checkBadReq(httptest.NewRequest(http.MethodPost, "/fake", nil), cc.handleInfo)
	// wrong data
	checkBadReq(httptest.NewRequest(http.MethodPost, "/fake", nil), cc.handleBanClient)
	checkBadReq(httptest.NewRequest(http.MethodPost, "/fake", nil), cc.handleUnbanClient)
	// json encode data error
	req := httptest.NewRequest(http.MethodGet, "/fake", nil)
	w := &badResponseWriter{FailTime: 1}
	cc.handleInfo(w, req)
	assert.Equal(w.StatusCode, http.StatusInternalServerError, "json encode error")
}

type badResponseWriter struct {
	FailTime   int
	Bytes      []byte
	StatusCode int
}

var _ http.ResponseWriter = (*badResponseWriter)(nil)

func (w *badResponseWriter) Header() http.Header {
	return nil
}

func (w *badResponseWriter) Write(bytes []byte) (int, error) {
	if w.FailTime > 0 {
		w.FailTime--
		return 0, errors.New("badResponseWriter will fail data write for several times")
	}
	w.Bytes = bytes
	return len(bytes), nil
}

func (w *badResponseWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}
