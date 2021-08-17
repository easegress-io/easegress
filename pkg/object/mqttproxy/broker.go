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
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

type Broker struct {
	name string
	spec *Spec
	rw   sync.RWMutex

	listener net.Listener
	backend  BackendMQ
	clients  map[string]*Client

	// done is the channel for shutdowning this proxy.
	done chan struct{}
}

func newBroker(spec *Spec) *Broker {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", spec.Port))
	if err != nil {
		logger.Errorf("%#v gen mqtt tcp listener, failed: %v", spec, err)
		return nil
	}

	broker := &Broker{
		name:     spec.Name,
		spec:     spec,
		listener: l,
		backend:  newBackendMQ(spec),
		clients:  make(map[string]*Client),
		done:     make(chan struct{}),
	}

	go broker.run()
	return broker
}

func (b *Broker) str() string {
	return fmt.Sprintf("%s cfg:%#v", b.name, b.spec)
}

func (b *Broker) run() {
	for {
		conn, err := b.listener.Accept()
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
	cid := client.info.cid

	b.rw.Lock()
	if oldClient, ok := b.clients[cid]; ok {
		oldClient.close()
	}
	b.clients[client.info.cid] = client
	b.rw.Unlock()

	client.readLoop()
}

func (b *Broker) close() {
	close(b.done)
	b.listener.Close()
	b.backend.close()

	b.rw.Lock()
	defer b.rw.Unlock()
	for _, v := range b.clients {
		v.close()
	}
	b.clients = nil

}
