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
	stdcontext "context"
	"errors"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func newContext(cid string, topic string) context.MQTTContext {
	client := &context.MockMQTTClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	return context.NewMQTTContext(stdcontext.Background(), client, packet)
}

func defaultFilterSpec(spec *Spec) *pipeline.FilterSpec {
	meta := &pipeline.FilterMetaSpec{
		Name:     "connect-control-demo",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
		Protocol: context.MQTT,
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

	spec := &Spec{
		BannedClients: []string{"banClient1", "banClient2"},
		BannedTopics:  []string{"banTopic1", "banTopic2"},
	}
	filterSpec := defaultFilterSpec(spec)
	cc.Init(filterSpec)
	newCc := &ConnectControl{}
	newCc.Inherit(filterSpec, cc)
	defer newCc.Close()
	status := newCc.Status().(*Status)
	assert.Equal(status.BannedClients, spec.BannedClients)
	assert.Equal(status.BannedTopics, spec.BannedTopics)
}

type testCase struct {
	cid        string
	topic      string
	err        error
	disconnect bool
	earlyStop  bool
}

func doTest(t *testing.T, spec *Spec, testCases []testCase) {
	assert := assert.New(t)
	filterSpec := defaultFilterSpec(spec)
	cc := &ConnectControl{}
	cc.Init(filterSpec)

	for _, test := range testCases {
		ctx := newContext(test.cid, test.topic)
		res := cc.HandleMQTT(ctx)
		assert.Equal(res.Err, test.err)
		assert.Equal(ctx.Disconnect(), test.disconnect)
		assert.Equal(ctx.EarlyStop(), test.earlyStop)
	}
	status := cc.Status().(*Status)
	assert.Equal(status.BannedClientRe, spec.BannedClientRe)
	assert.Equal(status.BannedTopicRe, spec.BannedTopicRe)
	assert.Equal(status.BannedClients, spec.BannedClients)
	assert.Equal(status.BannedTopics, spec.BannedTopics)
}

func TestHandleMQTT(t *testing.T) {
	// check BannedClients
	spec := &Spec{
		BannedClients: []string{"ban1", "ban2"},
		EarlyStop:     true,
	}
	testCases := []testCase{
		{cid: "ban1", topic: "unban", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "ban2", topic: "unban", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban1", topic: "ban/sport/ball", err: nil, disconnect: false, earlyStop: false},
		{cid: "unban2", topic: "ban/sport/run", err: nil, disconnect: false, earlyStop: false},
		{cid: "unban", topic: "unban", err: nil, disconnect: false, earlyStop: false},
	}
	doTest(t, spec, testCases)

	// check BannedTopics
	spec = &Spec{
		BannedTopics: []string{"ban/sport/ball", "ban/sport/run"},
		EarlyStop:    true,
	}
	testCases = []testCase{
		{cid: "unban1", topic: "ban/sport/ball", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban2", topic: "ban/sport/run", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "unban3", topic: "unban/sport", err: nil, disconnect: false, earlyStop: false},
		{cid: "unban4", topic: "unban", err: nil, disconnect: false, earlyStop: false},
	}
	doTest(t, spec, testCases)

	// check BannedClientRe
	spec = &Spec{
		BannedClientRe: "phone",
		EarlyStop:      true,
	}
	testCases = []testCase{
		{cid: "phone123", topic: "ban/sport/ball", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "phone256", topic: "ban/sport/run", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "tv", topic: "unban/sport", err: nil, disconnect: false, earlyStop: false},
		{cid: "tv", topic: "unban", err: nil, disconnect: false, earlyStop: false},
	}
	doTest(t, spec, testCases)

	// check BannedTopicRe
	spec = &Spec{
		BannedTopicRe: "sport",
		EarlyStop:     true,
	}
	testCases = []testCase{
		{cid: "phone123", topic: "ban/sport/ball", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "phone256", topic: "ban/sport/run", err: errors.New(resultBannedClientOrTopic), disconnect: true, earlyStop: true},
		{cid: "tv", topic: "unban", err: nil, disconnect: false, earlyStop: false},
		{cid: "tv", topic: "unban", err: nil, disconnect: false, earlyStop: false},
	}
	doTest(t, spec, testCases)

	// check re compile fail
	spec = &Spec{
		BannedClientRe: "(",
		BannedTopicRe:  "(?P<name>",
	}
	filterSpec := defaultFilterSpec(spec)
	cc := &ConnectControl{}
	cc.Init(filterSpec)
	assert.Nil(t, cc.bannedClientRe)
	assert.Nil(t, cc.bannedTopicRe)
}
