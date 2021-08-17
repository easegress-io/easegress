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

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

const (
	Connected    = 1
	Disconnected = 2
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
		broker *Broker
		conn   net.Conn

		info   ClientInfo
		status int
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
	} else {
		will = nil
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
	}
	return client
}

func (c *Client) readLoop() {
	for {
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
		// do nothing
	case *packets.SubackPacket:
		err = errors.New("broker not subscribe")
	case *packets.UnsubscribePacket:
		// do nothing
	case *packets.UnsubackPacket:
		err = errors.New("broker not unsubscribe")
	case *packets.PingreqPacket:
		// do nothing now
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
}

func (c *Client) close() {
	c.conn.Close()
}
