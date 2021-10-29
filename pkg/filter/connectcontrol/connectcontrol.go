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

package connectcontrol

import (
	"errors"
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of ConnectControl
	Kind = "ConnectControl"

	resultWrongPacket         = "wrongPacketError"
	resultBannedClientOrTopic = "bannedClientOrTopicError"
)

func init() {
	pipeline.Register(&ConnectControl{})
}

type (
	// ConnectControl is used to control MQTT clients connect status,
	// if MQTTContext ClientID in bannedClients, the connection will be closed,
	// if MQTTContext publish topic in bannedTopics, the connection will be closed.
	ConnectControl struct {
		filterSpec    *pipeline.FilterSpec
		spec          *Spec
		bannedClients sync.Map
		bannedTopics  sync.Map
	}

	// Spec describes the ConnectControl
	Spec struct {
		BannedClients []string `yaml:"bannedClients" jsonschema:"omitempty"`
		BannedTopics  []string `yaml:"bannedClients" jsonschema:"omitempty"`
		EarlyStop     bool     `yaml:"earlyStop" jsonschema:"omitempty"`
	}

	Status struct {
	}
)

var _ pipeline.Filter = (*ConnectControl)(nil)
var _ pipeline.MQTTFilter = (*ConnectControl)(nil)

// Kind return kind of ConnectControl
func (cc *ConnectControl) Kind() string {
	return Kind
}

// DefaultSpec return default spec of ConnectControl
func (cc *ConnectControl) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of ConnectControl
func (cc *ConnectControl) Description() string {
	return "ConnectControl control connections of MQTT clients"
}

// Results return results of ConnectControl
func (cc *ConnectControl) Results() []string {
	return nil
}

// Init init ConnectControl with pipeline filter spec
func (cc *ConnectControl) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	cc.filterSpec, cc.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	cc.reload()
}

func (cc *ConnectControl) reload() {
	for _, c := range cc.spec.BannedClients {
		cc.bannedClients.Store(c, struct{}{})
	}
	for _, t := range cc.spec.BannedTopics {
		cc.bannedTopics.Store(t, struct{}{})
	}
}

// Inherit init ConnectControl with previous generation
func (cc *ConnectControl) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	cc.Init(filterSpec)
}

// Status return status of ConnectControl
func (cc *ConnectControl) Status() interface{} {
	return nil
}

// Close close ConnectControl gracefully
func (cc *ConnectControl) Close() {
}

// HandleMQTT handle MQTT request
func (cc *ConnectControl) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{Err: errors.New(resultWrongPacket)}
	}

	cid := ctx.Client().ClientID()
	if _, ok := cc.bannedClients.Load(cid); !ok {
		topic := ctx.PublishPacket().TopicName
		if _, ok := cc.bannedTopics.Load(topic); !ok {
			return &context.MQTTResult{}
		}
	}

	ctx.SetDisconnect()
	if cc.spec.EarlyStop {
		ctx.SetEarlyStop()
	}
	return &context.MQTTResult{Err: errors.New(resultBannedClientOrTopic)}
}
