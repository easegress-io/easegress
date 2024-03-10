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

package connectcontrol

import (
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func newContext(cid string, topic string) *context.Context {
	ctx := context.New(nil)

	client := &mqttprot.MockClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	req := mqttprot.NewRequest(packet, client)
	ctx.SetRequest(context.DefaultNamespace, req)

	resp := mqttprot.NewResponse()
	ctx.SetOutputResponse(resp)
	return ctx
}

func defaultFilterSpec(spec *Spec) filters.Spec {
	spec.BaseSpec.MetaSpec.Kind = Kind
	spec.BaseSpec.MetaSpec.Name = "connect-control-demo"
	result, _ := filters.NewSpec(nil, "pipeline-demo", spec)
	return result
}

func TestConnectControl(t *testing.T) {
	assert := assert.New(t)

	cc := &ConnectControl{}
	assert.Equal(cc.Kind(), kind, "wrong kind")
	assert.NotEqual(len(cc.Kind().Description), 0, "description for ConnectControl is empty")

	assert.NotNil(cc.Kind().Results, "if update result, please update this case")

	spec := defaultFilterSpec(&Spec{
		BannedClients: []string{"banClient1", "banClient2"},
		BannedTopics:  []string{"banTopic1", "banTopic2"},
	}).(*Spec)
	cc = kind.CreateInstance(spec).(*ConnectControl)
	cc.Init()
	newCc := kind.CreateInstance(spec).(*ConnectControl)
	newCc.Inherit(cc)
	defer newCc.Close()
	status := newCc.Status().(*Status)
	assert.Equal(status.BannedClientNum, len(spec.BannedClients))
	assert.Equal(status.BannedTopicNum, len(spec.BannedTopics))
}

type testCase struct {
	cid        string
	topic      string
	errString  string
	disconnect bool
}

func doTest(t *testing.T, spec *Spec, testCases []testCase) {
	assert := assert.New(t)
	filterSpec := defaultFilterSpec(spec)
	cc := kind.CreateInstance(filterSpec).(*ConnectControl)
	cc.Init()

	for _, test := range testCases {
		ctx := newContext(test.cid, test.topic)
		res := cc.Handle(ctx)
		assert.Equal(res, test.errString)
		resp := ctx.GetOutputResponse().(*mqttprot.Response)
		assert.Equal(resp.Disconnect(), test.disconnect)
	}
	status := cc.Status().(*Status)
	assert.Equal(status.BannedClientRe, spec.BannedClientRe)
	assert.Equal(status.BannedTopicRe, spec.BannedTopicRe)
	assert.Equal(status.BannedClientNum, len(spec.BannedClients))
	assert.Equal(status.BannedTopicNum, len(spec.BannedTopics))
}

func TestHandle(t *testing.T) {
	// check BannedClients
	spec := &Spec{
		BannedClients: []string{"ban1", "ban2"},
	}
	testCases := []testCase{
		{cid: "ban1", topic: "unban", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "ban2", topic: "unban", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "unban1", topic: "ban/sport/ball", errString: "", disconnect: false},
		{cid: "unban2", topic: "ban/sport/run", errString: "", disconnect: false},
		{cid: "unban", topic: "unban", errString: "", disconnect: false},
	}
	doTest(t, spec, testCases)

	// check BannedTopics
	spec = &Spec{
		BannedTopics: []string{"ban/sport/ball", "ban/sport/run"},
	}
	testCases = []testCase{
		{cid: "unban1", topic: "ban/sport/ball", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "unban2", topic: "ban/sport/run", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "unban3", topic: "unban/sport", errString: "", disconnect: false},
		{cid: "unban4", topic: "unban", errString: "", disconnect: false},
	}
	doTest(t, spec, testCases)

	// check BannedClientRe
	spec = &Spec{
		BannedClientRe: "phone",
	}
	testCases = []testCase{
		{cid: "phone123", topic: "ban/sport/ball", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "phone256", topic: "ban/sport/run", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "tv", topic: "unban/sport", errString: "", disconnect: false},
		{cid: "tv", topic: "unban", errString: "", disconnect: false},
	}
	doTest(t, spec, testCases)

	// check BannedTopicRe
	spec = &Spec{
		BannedTopicRe: "sport",
	}
	testCases = []testCase{
		{cid: "phone123", topic: "ban/sport/ball", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "phone256", topic: "ban/sport/run", errString: resultBannedClientOrTopic, disconnect: true},
		{cid: "tv", topic: "unban", errString: "", disconnect: false},
		{cid: "tv", topic: "unban", errString: "", disconnect: false},
	}
	doTest(t, spec, testCases)

	// check re compile fail
	spec = &Spec{
		BannedClientRe: "(",
		BannedTopicRe:  "(?P<name>",
	}
	filterSpec := defaultFilterSpec(spec)
	cc := kind.CreateInstance(filterSpec).(*ConnectControl)
	cc.Init()
	assert.Nil(t, cc.bannedClientRe)
	assert.Nil(t, cc.bannedTopicRe)
}
