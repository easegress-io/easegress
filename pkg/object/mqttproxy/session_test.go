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
	"net"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/stretchr/testify/assert"
)

func TestSessionDoResend(t *testing.T) {
	assert := assert.New(t)

	// create broker
	mapper := &mockMuxMapper{}
	broker := getDefaultBroker(mapper)
	defer broker.close()

	// create client
	connect := &packets.ConnectPacket{
		ClientIdentifier: "client1",
		Username:         "user1",
		Password:         []byte("pass1"),
	}
	clientConn, testConn := net.Pipe()
	client := newClient(connect, broker, clientConn, nil)
	go client.writeLoop()

	broker.Lock()
	broker.clients[connect.ClientIdentifier] = client
	broker.Unlock()

	// create session
	broker.setSession(client, connect)
	sess := client.session

	// publish packet and recevie it
	go func() {
		sess.publish(nil, client, "topic1", []byte("payload1"), 1)
	}()
	p, err := packets.ReadPacket(testConn)
	assert.Nil(err)
	pub := p.(*packets.PublishPacket)
	assert.Equal("topic1", pub.TopicName)
	assert.Equal([]byte("payload1"), pub.Payload)

	// resend
	sess.Lock()
	assert.Equal(1, len(sess.pendingQueue))
	sess.Unlock()
	go func() {
		sess.doResend()
	}()
	p, err = packets.ReadPacket(testConn)
	assert.Nil(err)
	pub = p.(*packets.PublishPacket)
	assert.Equal("topic1", pub.TopicName)
	assert.Equal([]byte("payload1"), pub.Payload)
}
