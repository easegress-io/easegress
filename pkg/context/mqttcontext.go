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

package context

import (
	stdcontext "context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type (
	// MQTTContext is context for MQTT protocol
	MQTTContext interface {
		Context
		Client() MQTTClient
		Backend() MQTTBackend
		Cancel(error)
		Canceled() bool
		Duration() time.Duration
		Finish()

		PacketType() MQTTPacketType
		ConnectPacket() *packets.ConnectPacket       // read only
		PublishPacket() *packets.PublishPacket       // read only
		DisconnectPacket() *packets.DisconnectPacket // read only

		SetDrop()         // set drop value to true
		Drop() bool       // if true, this mqtt packet will be dropped
		SetDisconnect()   // set disconnect value to true
		Disconnect() bool // if true, this mqtt client will be disconnected
		SetEarlyStop()    // set early stop value to true
		EarlyStop() bool  // if early stop is true, pipeline will skip following filters and return
	}

	// MQTTBackend is backend of MQTT proxy
	MQTTBackend interface {
		Publish(target string, data []byte, headers map[string]string) error
	}

	// MQTTClient contains client info that send this packet
	MQTTClient interface {
		ClientID() string
		UserName() string
		Load(key interface{}) (value interface{}, ok bool)
		Store(key interface{}, value interface{})
		Delete(key interface{})
	}

	// MQTTPacketType contains supported mqtt packet type
	MQTTPacketType int

	mqttContext struct {
		mu         sync.RWMutex
		ctx        stdcontext.Context
		cancelFunc stdcontext.CancelFunc

		startTime        time.Time
		endTime          time.Time
		client           MQTTClient
		backend          MQTTBackend
		connectPacket    *packets.ConnectPacket
		publishPacket    *packets.PublishPacket
		disconnectPacket *packets.DisconnectPacket
		packetType       MQTTPacketType

		err        error
		drop       int32
		disconnect int32
		earlyStop  int32
	}

	// MQTTResult is result for handling mqtt request
	MQTTResult struct {
		Err error
	}
)

const (
	// MQTTConnect is mqtt packet type of connect
	MQTTConnect MQTTPacketType = 1

	// MQTTPublish is mqtt packet type of publish
	MQTTPublish MQTTPacketType = 2

	// MQTTDisconnect is mqtt packet type of disconnect
	MQTTDisconnect = 3

	// MQTTOther is all other mqtt packet type
	MQTTOther MQTTPacketType = 99
)

var _ MQTTContext = (*mqttContext)(nil)

// NewMQTTContext create new MQTTContext
func NewMQTTContext(ctx stdcontext.Context, backend MQTTBackend, client MQTTClient, packet packets.ControlPacket) MQTTContext {
	stdctx, cancelFunc := stdcontext.WithCancel(ctx)
	startTime := time.Now()
	mqttCtx := &mqttContext{
		ctx:        stdctx,
		cancelFunc: cancelFunc,
		startTime:  startTime,
		client:     client,
		backend:    backend,
	}

	switch packet := packet.(type) {
	case *packets.ConnectPacket:
		mqttCtx.connectPacket = packet
		mqttCtx.packetType = MQTTConnect
	case *packets.PublishPacket:
		mqttCtx.publishPacket = packet
		mqttCtx.packetType = MQTTPublish
	case *packets.DisconnectPacket:
		mqttCtx.disconnectPacket = packet
		mqttCtx.packetType = MQTTDisconnect
	default:
		mqttCtx.packetType = MQTTOther
	}
	return mqttCtx
}

// Protocol return protocol of mqttContext
func (ctx *mqttContext) Protocol() Protocol {
	return MQTT
}

// Deadline return deadline of mqttContext
func (ctx *mqttContext) Deadline() (time.Time, bool) {
	return ctx.ctx.Deadline()
}

// Done return done chan of mqttContext
func (ctx *mqttContext) Done() <-chan struct{} {
	return ctx.ctx.Done()
}

// Err return error of mqttContext
func (ctx *mqttContext) Err() error {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	if ctx.err != nil {
		return ctx.err
	}
	return ctx.ctx.Err()
}

// Value return value of mqttContext for given key
func (ctx *mqttContext) Value(key interface{}) interface{} {
	return ctx.ctx.Value(key)
}

// Client return mqttContext client
func (ctx *mqttContext) Client() MQTTClient {
	return ctx.client
}

// Cancel cancel mqttContext
func (ctx *mqttContext) Cancel(err error) {
	ctx.mu.Lock()
	if !ctx.canceled() {
		ctx.err = err
		ctx.cancelFunc()
	}
	ctx.mu.Unlock()
}

func (ctx *mqttContext) canceled() bool {
	return ctx.err != nil || ctx.ctx.Err() != nil
}

// Canceled return if mqttContext is canceled
func (ctx *mqttContext) Canceled() bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.canceled()
}

// Duration return time duration since this context start
func (ctx *mqttContext) Duration() time.Duration {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	if !ctx.endTime.IsZero() {
		return ctx.endTime.Sub(ctx.startTime)
	}
	return time.Since(ctx.startTime)
}

// Finish tell this context is finished
func (ctx *mqttContext) Finish() {
	ctx.mu.Lock()
	ctx.endTime = time.Now()
	ctx.mu.Unlock()
}

func (ctx *mqttContext) PacketType() MQTTPacketType {
	return ctx.packetType
}

func (ctx *mqttContext) ConnectPacket() *packets.ConnectPacket {
	return ctx.connectPacket
}

func (ctx *mqttContext) PublishPacket() *packets.PublishPacket {
	return ctx.publishPacket
}

func (ctx *mqttContext) DisconnectPacket() *packets.DisconnectPacket {
	return ctx.disconnectPacket
}

func (ctx *mqttContext) SetDrop() {
	atomic.StoreInt32(&ctx.drop, 1)
}

func (ctx *mqttContext) Drop() bool {
	return atomic.LoadInt32(&ctx.drop) == 1
}

func (ctx *mqttContext) SetDisconnect() {
	atomic.StoreInt32(&ctx.disconnect, 1)
}

func (ctx *mqttContext) Disconnect() bool {
	return atomic.LoadInt32(&ctx.disconnect) == 1
}

func (ctx *mqttContext) SetEarlyStop() {
	atomic.StoreInt32(&ctx.earlyStop, 1)
}

func (ctx *mqttContext) EarlyStop() bool {
	return atomic.LoadInt32(&ctx.earlyStop) == 1
}

func (ctx *mqttContext) Backend() MQTTBackend {
	return ctx.backend
}
