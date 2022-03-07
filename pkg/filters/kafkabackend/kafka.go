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
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "Kafka"

	resultParseErr = "parseErr"
)

func init() {
	pipeline.Register(&Kafka{})
}

type (
	// Kafka is kafka backend for MQTT proxy
	Kafka struct {
		filterSpec *pipeline.FilterSpec
		spec       *Spec
		producer   sarama.AsyncProducer
		done       chan struct{}
		header     string
	}
)

var _ pipeline.Filter = (*Kafka)(nil)

// Kind return kind of Kafka
func (k *Kafka) Kind() string {
	return Kind
}

// DefaultSpec return default spec of Kafka
func (k *Kafka) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of Kafka
func (k *Kafka) Description() string {
	return "Kafka is a backend of MQTTProxy"
}

// Results return possible results of Kafka
func (k *Kafka) Results() []string {
	return []string{resultParseErr}
}

func (k *Kafka) setHeader(spec *Spec) {
	if k.spec.Topic.Dynamic != nil {
		k.header = k.spec.Topic.Dynamic.Header
		if k.header == "" {
			panic("empty header")
		}
	}
}

// Init init Kafka
func (k *Kafka) Init(filterSpec *pipeline.FilterSpec) {
	k.filterSpec, k.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	k.done = make(chan struct{})
	k.setHeader(k.spec)

	config := sarama.NewConfig()
	config.ClientID = filterSpec.Name()
	config.Version = sarama.V1_0_0_0
	producer, err := sarama.NewAsyncProducer(k.spec.Backend, config)
	if err != nil {
		panic(fmt.Errorf("start sarama producer with address %v failed: %v", k.spec.Backend, err))
	}

	k.producer = producer
	go k.checkProduceError()
}

func (k *Kafka) checkProduceError() {
	for {
		select {
		case <-k.done:
			err := k.producer.Close()
			if err != nil {
				logger.Errorf("close kafka producer failed: %v", err)
			}
			return
		case err, ok := <-k.producer.Errors():
			if !ok {
				return
			}
			logger.Errorf("sarama producer failed: %v", err)
		}
	}
}

// Inherit init Kafka based on previous generation
func (k *Kafka) Inherit(filterSpec *pipeline.FilterSpec, previousGeneration pipeline.Filter) {
	previousGeneration.Close()
	k.Init(filterSpec)
}

// Close close Kafka
func (k *Kafka) Close() {
	close(k.done)
}

// Status return status of Kafka
func (k *Kafka) Status() interface{} {
	return nil
}

func (k *Kafka) getTopic(ctx context.HTTPContext) string {
	if k.header == "" {
		return k.spec.Topic.Default
	}
	topic := ctx.Request().Header().Get(http.CanonicalHeaderKey(k.header))
	if topic == "" {
		return k.spec.Topic.Default
	}
	return topic
}

func (k *Kafka) handle(ctx context.HTTPContext) (result string) {
	topic := k.getTopic(ctx)

	body, err := ioutil.ReadAll(ctx.Request().Body())
	if err != nil {
		return resultParseErr
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(body),
	}
	k.producer.Input() <- msg
	return ""
}

// Handle handles HTTPContext.
func (k *Kafka) Handle(ctx context.HTTPContext) (result string) {
	result = k.handle(ctx)
	return ctx.CallNextHandler(result)
}
