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
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type (
	// MQTTContext is context for MQTT protocol
	MQTTContext interface {
		Context
		Client() MQTTClient
		Topic() string
		Payload() []byte
		Cancel(error)
		Canceled() bool
		Duration() time.Duration
		Finish()
	}

	// MQTTClient contains client info that send this packet
	MQTTClient interface {
		ClientID() string
		UserName() string
	}

	mqttContext struct {
		mu         sync.RWMutex
		ctx        stdcontext.Context
		cancelFunc stdcontext.CancelFunc

		startTime *time.Time
		endTime   *time.Time
		client    MQTTClient
		packet    *packets.PublishPacket
		err       error
	}
)

var _ MQTTContext = (*mqttContext)(nil)

func NewMQTTContext(ctx stdcontext.Context, client MQTTClient, packet *packets.PublishPacket) MQTTContext {
	stdctx, cancelFunc := stdcontext.WithCancel(ctx)
	startTime := time.Now()

	return &mqttContext{
		ctx:        stdctx,
		cancelFunc: cancelFunc,
		startTime:  &startTime,
		client:     client,
		packet:     packet,
	}
}

func (ctx *mqttContext) Protocol() Protocol {
	return MQTT
}

func (ctx *mqttContext) Deadline() (time.Time, bool) {
	return ctx.ctx.Deadline()
}

func (ctx *mqttContext) Done() <-chan struct{} {
	return ctx.ctx.Done()
}

func (ctx *mqttContext) Err() error {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	if ctx.err != nil {
		return ctx.err
	}
	return ctx.ctx.Err()
}

func (ctx *mqttContext) Value(key interface{}) interface{} {
	return ctx.ctx.Value(key)
}

func (ctx *mqttContext) Client() MQTTClient {
	return ctx.client
}

func (ctx *mqttContext) Topic() string {
	return ctx.packet.TopicName
}

func (ctx *mqttContext) Payload() []byte {
	return ctx.packet.Payload
}

func (ctx *mqttContext) Cancel(err error) {
	ctx.mu.Lock()
	if !ctx.Canceled() {
		ctx.err = err
		ctx.cancelFunc()
	}
	ctx.mu.Unlock()
}

func (ctx *mqttContext) Canceled() bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.err != nil || ctx.ctx.Err() != nil
}

func (ctx *mqttContext) Duration() time.Duration {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	if ctx.endTime != nil {
		return ctx.endTime.Sub(*ctx.startTime)
	}
	return time.Since(*ctx.startTime)
}

func (ctx *mqttContext) Finish() {
	ctx.mu.Lock()
	endTime := time.Now()
	ctx.endTime = &endTime
	ctx.mu.Unlock()
}
