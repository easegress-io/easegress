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

	// a := assert.New(t)

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
