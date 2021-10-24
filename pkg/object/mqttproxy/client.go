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
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/logger"
)

const (
	// Connected is MQTT client status of Connected
	Connected = 1
	// Disconnected is MQTT client status of Disconnected
	Disconnected = 2

	// QoS0 for "At most once"
	QoS0 byte = 0
	// QoS1 for "At least once
	QoS1 byte = 1
	// QoS2 for "Exactly once"
	QoS2 byte = 2
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

		info       ClientInfo
		statusFlag int32
		writeCh    chan packets.ControlPacket
		done       chan struct{}
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
		broker:     broker,
		conn:       conn,
		info:       info,
		statusFlag: Connected,
		writeCh:    make(chan packets.ControlPacket, 50),
		done:       make(chan struct{}),
	}
	return client
}

func (c *Client) readLoop() {
	defer func() {
		if c.info.will != nil {
			c.broker.backend.publish(c.info.will)
		}
		c.closeAndDelSession()
		c.broker.removeClient(c.info.cid)
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

		logger.Debugf("client %s readLoop read packet", c.info.cid)
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
		c.processPublish(p)
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

func (c *Client) processPublish(publish *packets.PublishPacket) {
	logger.Debugf("client %s process publish %v", c.info.cid, publish.TopicName)
	err := c.broker.backend.publish(publish)
	if err != nil {
		logger.Errorf("client %v publish %v failed: %v", c.info.cid, publish.TopicName, err)
	}
	switch publish.Qos {
	case QoS0:
		// do nothing
	case QoS1:
		puback := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
		puback.MessageID = publish.MessageID
		c.writePacket(puback)
	case QoS2:
		// not support yet
	}
}

func (c *Client) processPuback(puback *packets.PubackPacket) {
	c.session.puback(puback)
}

func (c *Client) processSubscribe(packet *packets.SubscribePacket) {
	logger.Debugf("client %s processSubscribe %v", c.info.cid, packet.Topics)
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
	logger.Debugf("client %s processUnsubscribe %v", c.info.cid, packet.Topics)
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

func (c *Client) writePacket(packet packets.ControlPacket) {
	c.writeCh <- packet
}

func (c *Client) writeLoop() {
	for {
		select {
		case p := <-c.writeCh:
			err := p.Write(c.conn)
			if err != nil {
				logger.Errorf("write puback to client %s failed: %s", c.info.cid, err)
				c.closeAndDelSession()
			}
		case <-c.done:
			return
		}
	}
}

func (c *Client) close() {
	c.Lock()
	defer c.Unlock()
	if c.disconnected() {
		return
	}
	atomic.StoreInt32(&c.statusFlag, Disconnected)
	close(c.done)
}

func (c *Client) disconnected() bool {
	return atomic.LoadInt32(&c.statusFlag) == Disconnected
}

func (c *Client) closeAndDelSession() {
	c.broker.sessMgr.delLocal(c.info.cid)
	if c.session.cleanSession() {
		c.broker.sessMgr.delDB(c.info.cid)
	}

	topics, _, _ := c.session.allSubscribes()
	c.broker.topicMgr.unsubscribe(topics, c.info.cid)

	c.close()
}
