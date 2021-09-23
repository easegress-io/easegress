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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

const (
	// Connected is MQTT client status of Connected
	Connected = 1
	// Disconnected is MQTT client status of Disconnected
	Disconnected = 2

	// Qos0 for "At most once"
	Qos0 byte = 0
	// Qos1 for "At least once
	Qos1 byte = 1
	// Qos2 for "Exactly once"
	Qos2 byte = 2
)

type (
	// ClientInfo is basic infomation for client
	ClientInfo struct {
		cid       string
		username  string
		password  string
		keepalive uint16
		will      *packets.PublishPacket
	}

	// Client represents a MQTT client connection in Broker
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
		will = packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
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
	defer func() {
		if c.info.will != nil {
			c.broker.backend.publish(c.info.will)
		}
		c.close()
	}()
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
				logger.Errorf("set read timeout failed: %s", c.info.cid)
			}
		}

		packet, err := packets.ReadPacket(c.conn)
		if err != nil {
			logger.Errorf("client %s read packet failed: %v", c.info.cid, err)
			return
		}
		if _, ok := packet.(*packets.DisconnectPacket); ok {
			c.info.will = nil
			return
		}
		err = c.processPacket(packet)
		if err != nil {
			logger.Errorf("client %s process packet failed: %v", c.info.cid, err)
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
		err = c.processPublish(p)
	case *packets.PubackPacket:
		c.processPuback(p)
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
	default:
		err = errors.New("unknown packet")
	}
	return err
}

func (c *Client) processPublish(publish *packets.PublishPacket) error {
	c.broker.backend.publish(publish)
	switch publish.Qos {
	case Qos0:
		// do nothing
	case Qos1:
		puback := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		puback.MessageID = publish.MessageID
		err := c.writePacket(puback)
		if err != nil {
			return fmt.Errorf("write puback to client %s failed: %s", c.info.cid, err)
		}
	case Qos2:
		// not support yet
	}
	return nil
}

func (c *Client) processPuback(puback *packets.PubackPacket) {
	c.session.puback(puback)
}

func (c *Client) processSubscribe(packet *packets.SubscribePacket) {
	err := c.broker.topicMgr.subscribe(packet.Topics, packet.Qoss, c.info.cid)
	if err != nil {
		logger.Errorf("client %v subscribe %v failed: %v", c.info.cid, packet.Topics, err)
		return
	}
	c.session.subscribe(packet.Topics, packet.Qoss)

	suback := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
	suback.MessageID = packet.MessageID
	suback.ReturnCodes = make([]byte, len(packet.Topics))
	for i := range packet.Topics {
		suback.ReturnCodes[i] = packet.Qos
	}
	c.writePacket(suback)
}

func (c *Client) processUnsubscribe(packet *packets.UnsubscribePacket) {
	err := c.broker.topicMgr.unsubscribe(packet.Topics, c.info.cid)
	if err != nil {
		logger.Errorf("client %v unsubscribe %v failed: %v", c.info.cid, packet.Topics, err)
	}
	c.session.unsubscribe(packet.Topics)

	unsuback := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
	unsuback.MessageID = packet.MessageID
	c.writePacket(unsuback)
}

func (c *Client) processPingreq(packet *packets.PingreqPacket) {
	resp := packets.NewControlPacket(packets.Pingresp).(*packets.PingrespPacket)
	c.writePacket(resp)
}

func (c *Client) writePacket(packet packets.ControlPacket) error {
	c.Lock()
	defer c.Unlock()
	return packet.Write(c.conn)
}

func (c *Client) close() {
	c.Lock()
	defer c.Unlock()
	if c.status == Disconnected {
		return
	}
	c.status = Disconnected
	close(c.done)
	c.broker.sessMgr.delLocal(c.info.cid)
}
