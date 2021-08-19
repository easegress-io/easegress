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
	"errors"
	"net"
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

const (
	Connected    = 1
	Disconnected = 2

	// three qualities of service for message delivery:
	Qos0 byte = 0 // for "At most once"
	Qos1 byte = 1 // for "At least once
	Qos2 byte = 2 // for "Exactly once"
)

type (
	ClientInfo struct {
		cid       string
		username  string
		password  string
		keepalive uint16
		will      *packets.PublishPacket
	}

	Client struct {
		sync.Mutex

		broker  *Broker
		session *Session
		conn    net.Conn

		info   ClientInfo
		status int
		done   chan struct{}
	}
)

func newClient(connect *packets.ConnectPacket, broker *Broker, conn net.Conn) *Client {
	var will *packets.PublishPacket
	if connect.WillFlag {
		will := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
		will.Qos = connect.WillQos
		will.TopicName = connect.WillTopic
		will.Retain = connect.WillRetain
		will.Payload = connect.WillMessage
		will.Dup = connect.Dup
	}

	info := ClientInfo{
		cid:       connect.ClientIdentifier,
		username:  connect.Username,
		password:  string(connect.Password),
		keepalive: connect.Keepalive,
		will:      will,
	}
	client := &Client{
		broker: broker,
		conn:   conn,
		info:   info,
		status: Connected,
		done:   make(chan struct{}),
	}
	return client
}

func (c *Client) readLoop() {
	defer c.close()
	keepAlive := time.Duration(c.info.keepalive) * time.Second
	timeOut := keepAlive + keepAlive/2
	for {
		select {
		case <-c.done:
			return
		default:
		}

		if keepAlive > 0 {
			if err := c.conn.SetDeadline(time.Now().Add(timeOut)); err != nil {
				logger.Errorf("set read timeout error: %s", c.info.cid)
			}
		}

		packet, err := packets.ReadPacket(c.conn)
		if err != nil {
			logger.Errorf("%s, client %s, read packet error: %s", c.broker.str(), c.info.cid, err)
			return
		}
		err = c.processPacket(packet)
		if err != nil {
			logger.Errorf("%s, client %s, error: %s", c.broker.str(), c.info.cid, err)
			return
		}
	}
}

func (c *Client) processPacket(packet packets.ControlPacket) error {
	var err error
	switch p := packet.(type) {
	case *packets.ConnectPacket:
		err = errors.New("double connect")
	case *packets.ConnackPacket:
		err = errors.New("client send connack")
	case *packets.PublishPacket:
		c.processPublish(p)
	case *packets.PubackPacket:
		// TODO after add http endpoint we can receive puback
		err = errors.New("broker now not publish to client")
	case *packets.PubrecPacket, *packets.PubrelPacket, *packets.PubcompPacket:
		err = errors.New("qos2 not support now")
	case *packets.SubscribePacket:
		c.processSubscribe(p)
	case *packets.SubackPacket:
		err = errors.New("broker not subscribe")
	case *packets.UnsubscribePacket:
		c.processUnsubscribe(p)
	case *packets.UnsubackPacket:
		err = errors.New("broker not unsubscribe")
	case *packets.PingreqPacket:
		c.processPingreq(p)
	case *packets.PingrespPacket:
		err = errors.New("broker not ping")
	case *packets.DisconnectPacket:
		err = errors.New("client disconnect")
	default:
		err = errors.New("unknown packet")
	}
	return err
}

func (c *Client) processPublish(publish *packets.PublishPacket) {
	c.broker.backend.publish(publish)
	switch publish.Qos {
	case Qos0:
		// do nothing
	case Qos1:
		puback := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		puback.MessageID = publish.MessageID
		err := puback.Write(c.conn)
		if err != nil {
			logger.Errorf("write puback to client %s failed: %s", c.info.cid, err)
		}
	case Qos2:
		// not support yet
	}
}

func (c *Client) processSubscribe(packet *packets.SubscribePacket) {
	c.session.subscribe(packet.Topics, packet.Qoss)

	suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
	suback.MessageID = packet.MessageID
	suback.ReturnCodes = make([]byte, len(packet.Topics))
	for i := range packet.Topics {
		suback.ReturnCodes[i] = packet.Qos
	}
	suback.Write(c.conn)
}

func (c *Client) processUnsubscribe(packet *packets.UnsubscribePacket) {
	c.session.unsubscribe(packet.Topics)

	unsuback := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
	unsuback.MessageID = packet.MessageID
	unsuback.Write(c.conn)
}

func (c *Client) processPingreq(packet *packets.PingreqPacket) {
	resp := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
	resp.Write(c.conn)
}

func (c *Client) close() {
	c.Lock()
	defer c.Unlock()
	if c.status == Disconnected {
		return
	}
	c.status = Disconnected
	close(c.done)
	if c.session.cleanSession() {
		c.broker.sessMgr.del(c.info.cid)
	}
}
