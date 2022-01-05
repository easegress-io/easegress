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

package kafka

import (
	"errors"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "Kafka"

	resultMQTTTopicMapFailed = "MQTTTopicMapFailed"
)

var errMQTTTopicMapFailed = errors.New(resultMQTTTopicMapFailed)

func init() {
	pipeline.Register(&Kafka{})
}

type (
	Kafka struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		mapFunc    topicMapFunc
		producer   sarama.AsyncProducer
		done       chan struct{}
	}
)

var _ pipeline.Filter = (*Kafka)(nil)
var _ pipeline.MQTTFilter = (*Kafka)(nil)

func (k *Kafka) Kind() string {
	return Kind
}

func (k *Kafka) DefaultSpec() interface{} {
	return &Spec{}
}

func (k *Kafka) Description() string {
	return "Kafka is a backend of MQTTProxy"
}

func (k *Kafka) Results() []string {
	return []string{resultMQTTTopicMapFailed}
}

func (k *Kafka) Init(filterSpec *pipeline.FilterSpec) {
	if filterSpec.Protocol() != context.MQTT {
		panic("filter ConnectControl only support MQTT protocol for now")
	}
	k.filterSpec, k.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	k.mapFunc = getTopicMapFunc(k.spec.TopicMapper)
	k.done = make(chan struct{})

	config := sarama.NewConfig()
	config.ClientID = filterSpec.Name()
	config.Version = sarama.V1_0_0_0
	producer, err := sarama.NewAsyncProducer(k.spec.Backend, config)
	if err != nil {
		panic(fmt.Errorf("start sarama producer with address %v failed: %v", k.spec.Backend, err))
	}

	go func() {
		for {
			select {
			case <-k.done:
				return
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				logger.SpanErrorf(nil, "sarama producer failed: %v", err)
			}
		}
	}()

	k.producer = producer
}

func (k *Kafka) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	k.Init(filterSpec)
}

func (k *Kafka) Close() {
	close(k.done)
	err := k.producer.Close()
	if err != nil {
		logger.Errorf("close kafka producer failed: %v", err)
	}
}

func (k *Kafka) Status() interface{} {
	return nil
}

func (k *Kafka) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	if ctx.PacketType() != context.MQTTPublish {
		return &context.MQTTResult{}
	}
	p := ctx.PublishPacket()
	var msg *sarama.ProducerMessage
	logger.Debugf("produce msg with topic %s", p.TopicName)

	if k.mapFunc != nil {
		topic, headers, err := k.mapFunc(p.TopicName)
		if err != nil {
			logger.Errorf("packet TopicName %s not match TopicMapper rules", p.TopicName)
			return &context.MQTTResult{Err: errMQTTTopicMapFailed}
		}
		kafkaHeaders := []sarama.RecordHeader{}
		for k, v := range headers {
			kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
		}

		msg = &sarama.ProducerMessage{
			Topic:   topic,
			Headers: kafkaHeaders,
			Value:   sarama.ByteEncoder(p.Payload),
		}
	} else {
		msg = &sarama.ProducerMessage{
			Topic: p.TopicName,
			Value: sarama.ByteEncoder(p.Payload),
		}
	}
	k.producer.Input() <- msg
	return &context.MQTTResult{}
}
