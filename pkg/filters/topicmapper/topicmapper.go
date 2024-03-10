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

// Package topicmapper maps MQTT topic to Kafka topics and key-value headers
package topicmapper

import (
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
)

const (
	// Kind is the kind of TopicMapper
	Kind = "TopicMapper"

	resultMQTTTopicMapFailed = "MQTTTopicMapFailed"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "TopicMapper maps MQTT topic to Kafka topics and key-value headers",
	Results:     []string{resultMQTTTopicMapFailed},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &TopicMapper{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// TopicMapper map MQTT multi-level topic into topic and key-value headers
	TopicMapper struct {
		spec  *Spec
		mapFn topicMapFunc
	}
)

var _ filters.Filter = (*TopicMapper)(nil)

// Name returns the name of the TopicMapper filter instance.
func (k *TopicMapper) Name() string {
	return k.spec.Name()
}

// Kind return kind of TopicMapper
func (k *TopicMapper) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the TopicMapper
func (k *TopicMapper) Spec() filters.Spec {
	return k.spec
}

// Init init TopicMapper
func (k *TopicMapper) Init() {
	k.mapFn = getTopicMapFunc(k.spec)
	if k.mapFn == nil {
		panic("invalid spec for TopicMapper")
	}
}

// Inherit init TopicMapper based on previous generation
func (k *TopicMapper) Inherit(previousGeneration filters.Filter) {
	k.Init()
}

// Close close TopicMapper
func (k *TopicMapper) Close() {
}

// Status return status of TopicMapper
func (k *TopicMapper) Status() interface{} {
	return nil
}

// Handle handle context
func (k *TopicMapper) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*mqttprot.Request)
	if req.PacketType() != mqttprot.PublishType {
		return ""
	}

	publish := req.PublishPacket()
	topic, headers, err := k.mapFn(publish.TopicName)
	if err != nil {
		logger.Errorf("map topic %v failed, %v", publish.TopicName, err)

		return resultMQTTTopicMapFailed
	}
	ctx.SetData(k.spec.SetKV.Topic, topic)
	ctx.SetData(k.spec.SetKV.Headers, headers)
	return ""
}
