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

// Package kafka implements a kafka proxy for MQTT requests.
package kafka

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
)

const (
	// Kind is the kind of Kafka
	Kind = "KafkaMQTT"

	resultGetDataFailed = "getDataFailed"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Kafka is a kafka proxy for MQTT requests",
	Results:     []string{resultGetDataFailed},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Kafka{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Kafka is a kafka proxy for MQTT requests.
	Kafka struct {
		spec     *Spec
		producer sarama.AsyncProducer
		done     chan struct{}

		defaultTopic string
		topicKey     string
		headerKey    string
		payloadKey   string
	}
)

var _ filters.Filter = (*Kafka)(nil)

// Name returns the name of the Kafka filter instance.
func (k *Kafka) Name() string {
	return k.spec.Name()
}

// Kind return kind of Kafka
func (k *Kafka) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Kafka
func (k *Kafka) Spec() filters.Spec {
	return k.spec
}

func (k *Kafka) setKV() {
	kv := k.spec.KVMap
	if kv != nil {
		k.topicKey = kv.TopicKey
		k.headerKey = kv.HeaderKey
		k.payloadKey = kv.PayloadKey
	}
	if k.spec.Topic != nil {
		k.defaultTopic = k.spec.Topic.Default
	}
}

var newAsyncProducer = sarama.NewAsyncProducer

func (k *Kafka) setProducer() {
	config := sarama.NewConfig()
	config.ClientID = k.spec.Name()
	config.Version = sarama.V1_0_0_0
	producer, err := newAsyncProducer(k.spec.Backend, config)
	if err != nil {
		panic(fmt.Errorf("start sarama producer with address %v failed: %v", k.spec.Backend, err))
	}
	k.producer = producer

	go func() {
		for {
			select {
			case <-k.done:
				err := producer.Close()
				if err != nil {
					logger.Errorf("close kafka producer failed: %v", err)
				}
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.SpanErrorf(nil, "sarama producer failed: %v", err)
			}
		}
	}()
}

// Init init Kafka
func (k *Kafka) Init() {
	k.done = make(chan struct{})
	k.setKV()
	k.setProducer()
}

// Inherit init Kafka based on previous generation
func (k *Kafka) Inherit(previousGeneration filters.Filter) {
	k.Init()
}

// Close close Kafka
func (k *Kafka) Close() {
	close(k.done)
}

// Status return status of Kafka
func (k *Kafka) Status() interface{} {
	return nil
}

// Handle handles context
func (k *Kafka) Handle(ctx *context.Context) string {
	var topic string
	var payload []byte
	var ok bool

	// set data from kv map
	if k.topicKey != "" {
		topic, ok = ctx.GetData(k.topicKey).(string)
		if !ok {
			return resultGetDataFailed
		}
	}
	var headerFromData map[string]string
	if k.headerKey != "" {
		headerFromData, ok = ctx.GetData(k.headerKey).(map[string]string)
		if !ok {
			return resultGetDataFailed
		}
	}
	if k.payloadKey != "" {
		payload, ok = ctx.GetData(k.payloadKey).([]byte)
		if !ok {
			return resultGetDataFailed
		}
	}

	req := ctx.GetInputRequest().(*mqttprot.Request)
	headers := map[string]string{}
	// set data from PublishPacket if data is missing
	headers["clientID"] = req.Client().ClientID()
	headers["username"] = req.Client().UserName()
	if req.PacketType() == mqttprot.PublishType {
		p := req.PublishPacket()
		headers["mqttTopic"] = p.TopicName
		if topic == "" {
			topic = p.TopicName
		}
		if payload == nil {
			payload = p.Payload
		}
	}
	for k, v := range headerFromData {
		headers[k] = v
	}

	if topic == "" {
		topic = k.defaultTopic
	}

	if topic == "" || payload == nil {
		return resultGetDataFailed
	}

	kafkaHeaders := []sarama.RecordHeader{}
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}

	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Headers: kafkaHeaders,
		Value:   sarama.ByteEncoder(payload),
	}
	k.producer.Input() <- msg
	return ""
}
