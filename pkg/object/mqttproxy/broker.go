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
	"fmt"
	"net"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type Broker struct {
	name string
	spec *Spec

	listener net.Listener
	backend  BackendMQ
	// clients  sync.Map

	// done is the channel for shutdowning this proxy.
	done chan struct{}
}

func newBroker(name string, spec *Spec) *Broker {
	broker := &Broker{
		name:    name,
		spec:    spec,
		done:    make(chan struct{}),
		backend: newBackendMQ(spec),
	}
	go broker.run()
	return broker
}

func (b *Broker) str() string {
	return fmt.Sprintf("%s cfg:%#v", b.name, b.spec)
}

func (b *Broker) run() {
	addr := fmt.Sprintf(":%d", b.spec.Port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Errorf("%s gen mqtt tcp listener, failed: %v", b.str(), err)
		return
	}
	b.listener = l

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-b.done:
				return
			default:
				logger.Errorf("%s net listener accept err: %s", b.str(), err)
			}
		} else {
			go b.handleConn(conn)
		}
	}
}

func (b *Broker) checkClientAuth(connect *packets.ConnectPacket) bool {
	// do auth here use
	// TODO add real client auth here
	// mock here
	cid := connect.ClientIdentifier
	name := connect.Username
	passwd := string(connect.Password)
	return cid != "" && name == "test" && passwd == "test"
}

func (b *Broker) handleConn(conn net.Conn) {
	defer conn.Close()
	packet, err := packets.ReadPacket(conn)
	if err != nil {
		logger.Errorf("%s read connect packet error: %s", b.str(), err)
		return
	}
	connect, ok := packet.(*packets.ConnectPacket)
	if !ok {
		logger.Errorf("received %s that was not Connect", packet.String())
		return
	}

	connack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	connack.SessionPresent = connect.CleanSession
	connack.ReturnCode = connect.Validate()
	if connack.ReturnCode != packets.Accepted {
		err = connack.Write(conn)
		logger.Errorf("%s, unvalid connection %#v, write back err %s", b.str(), connack.ReturnCode, err)
		return
	}

	if !b.checkClientAuth(connect) {
		connack.ReturnCode = packets.ErrRefusedNotAuthorised
		err = connack.Write(conn)
		logger.Errorf("%s, unvalid connection %#v, write back err %s", b.str(), connack.ReturnCode, err)
		return
	}

	err = connack.Write(conn)
	if err != nil {
		logger.Errorf("%s, send connack to client %s err %s", b.str(), connect.ClientIdentifier, err)
		return
	}

	client := newClient(connect, b, conn)
	// TODO session for later
	// TODO add client to broker and replace old client graceful
	client.readLoop()
}

func (b *Broker) close() {
	close(b.done)
	b.listener.Close()
	// TODO: graceful shutdown later
}
