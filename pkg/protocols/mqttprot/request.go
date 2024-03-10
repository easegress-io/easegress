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

package mqttprot

import (
	"bytes"
	"io"

	"github.com/megaease/easegress/v2/pkg/protocols"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type (
	// Request contains MQTT packet.
	Request struct {
		client     Client
		packet     packets.ControlPacket
		packetType PacketType
		payload    []byte
	}

	// Client contains MQTT client info that send this packet
	Client interface {
		ClientID() string
		UserName() string
		Load(key interface{}) (value interface{}, ok bool)
		Store(key interface{}, value interface{})
		Delete(key interface{})
	}

	// PacketType contains supported MQTT packet type
	PacketType int
)

const (
	// ConnectType is MQTT packet type of connect
	ConnectType PacketType = 1

	// PublishType is MQTT packet type of publish
	PublishType PacketType = 2

	// DisconnectType is MQTT packet type of disconnect
	DisconnectType PacketType = 3

	// SubscribeType is MQTT packet type of subscribe
	SubscribeType PacketType = 4

	// UnsubscribeType is MQTT packet type of unsubscribe
	UnsubscribeType PacketType = 5

	// OtherType is all other MQTT packet type
	OtherType PacketType = 99
)

var _ protocols.Request = (*Request)(nil)

// NewRequest create new MQTT Request
func NewRequest(packet packets.ControlPacket, client Client) *Request {
	req := &Request{
		client: client,
		packet: packet,
	}
	switch p := packet.(type) {
	case *packets.ConnectPacket:
		req.packetType = ConnectType
	case *packets.PublishPacket:
		req.packetType = PublishType
		req.payload = p.Payload
	case *packets.DisconnectPacket:
		req.packetType = DisconnectType
	case *packets.SubscribePacket:
		req.packetType = SubscribeType
	case *packets.UnsubscribePacket:
		req.packetType = UnsubscribeType
	default:
		req.packetType = OtherType
	}
	return req
}

// IsStream returns whether the payload of the request is a stream.
func (r *Request) IsStream() bool {
	return false
}

// Client return MQTT request client
func (r *Request) Client() Client {
	return r.client
}

// PacketType return MQTT request packet type
func (r *Request) PacketType() PacketType {
	return r.packetType
}

// ConnectPacket return MQTT connect packet if PacketType is ConnectType
func (r *Request) ConnectPacket() *packets.ConnectPacket {
	return r.packet.(*packets.ConnectPacket)
}

// PublishPacket return MQTT publish packet if PacketType is PublishType
func (r *Request) PublishPacket() *packets.PublishPacket {
	return r.packet.(*packets.PublishPacket)
}

// DisconnectPacket return MQTT disconnect packet if PacketType is DisconnectType
func (r *Request) DisconnectPacket() *packets.DisconnectPacket {
	return r.packet.(*packets.DisconnectPacket)
}

// SubscribePacket return MQTT subscribe packet if PacketType is SubscribeType
func (r *Request) SubscribePacket() *packets.SubscribePacket {
	return r.packet.(*packets.SubscribePacket)
}

// UnsubscribePacket return MQTT unsubscribe packet if PacketType is UnsubscribeType
func (r *Request) UnsubscribePacket() *packets.UnsubscribePacket {
	return r.packet.(*packets.UnsubscribePacket)
}

// Header return MQTT request header
func (r *Request) Header() protocols.Header {
	// TODO: what header to return?
	return nil
}

// RealIP returns the real IP of the request.
func (r *Request) RealIP() string {
	panic("not implemented")
}

// SetPayload set the payload of the request to payload.
func (r *Request) SetPayload(payload interface{}) {
	p, ok := payload.([]byte)
	if !ok {
		panic("payload is not a byte slice")
	}
	r.payload = p
	if r.packetType == PublishType {
		r.packet.(*packets.PublishPacket).Payload = p
	}
}

// GetPayload returns a new payload reader.
func (r *Request) GetPayload() io.Reader {
	return bytes.NewReader(r.payload)
}

// RawPayload returns the payload in []byte, the caller should
// not modify its content.
func (r *Request) RawPayload() []byte {
	return r.payload
}

// PayloadSize returns the length of the payload.
func (r *Request) PayloadSize() int64 {
	return int64(len(r.payload))
}

// Close closes the request.
func (r *Request) Close() {
}

// ToBuilderRequest wraps the request and returns the wrapper, the
// return value can be used in the template of the Builder filters.
func (r *Request) ToBuilderRequest(name string) interface{} {
	panic("not implemented")
}

// NewRequestInfo returns a new requestInfo.
func (p *Protocol) NewRequestInfo() interface{} {
	panic("not implemented")
}

// BuildRequest builds and returns a request according to the given reqInfo.
func (p *Protocol) BuildRequest(reqInfo interface{}) (protocols.Request, error) {
	panic("not implemented")
}
