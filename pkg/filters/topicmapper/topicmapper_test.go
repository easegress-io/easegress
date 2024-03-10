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

package topicmapper

import (
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
	"github.com/stretchr/testify/assert"
)

func newContext(cid string, topic string) *context.Context {
	ctx := context.New(nil)

	client := &mqttprot.MockClient{
		MockClientID: cid,
	}
	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic

	req := mqttprot.NewRequest(packet, client)
	ctx.SetInputRequest(req)
	return ctx
}

func TestTopicMapper(t *testing.T) {
	spec := getDefaultSpec()
	topicMapper := kind.CreateInstance(spec)
	topicMapper.Init()
	defer topicMapper.Close()

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
		ctx := newContext("client", tt.mqttTopic)
		topicMapper.Handle(ctx)
		assert.Equal(t, tt.topic, ctx.GetData("topic").(string))
		assert.Equal(t, tt.headers, ctx.GetData("headers").(map[string]string))
	}
}
