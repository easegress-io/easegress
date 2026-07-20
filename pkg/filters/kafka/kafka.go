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
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/mqttprot"
)

const (
	// Kind is the kind of Kafka
	Kind = "KafkaMQTT"

	resultGetDataFailed       = "getDataFailed"
	resultProducerUnavailable = "producerUnavailable"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Kafka is a kafka proxy for MQTT requests",
	Results:     []string{resultGetDataFailed, resultProducerUnavailable},
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
		spec      *Spec
		producer  sarama.AsyncProducer
		done      chan struct{}
		closeOnce sync.Once

		mu           sync.RWMutex
		reconnecting int32
		lastErr      string

		defaultTopic string
		topicKey     string
		headerKey    string
		payloadKey   string
	}

	// Status is the runtime status of Kafka.
	Status struct {
		Health string `json:"health"`
		Ready  bool   `json:"ready"`
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

var (
	newAsyncProducer        = sarama.NewAsyncProducer
	producerRetryBackoff    = time.Second
	producerMaxRetryBackoff = 30 * time.Second
)

func (k *Kafka) setProducer() error {
	config := sarama.NewConfig()
	config.ClientID = k.spec.Name()
	config.Version = sarama.V1_0_0_0
	producer, err := newAsyncProducer(k.spec.Backend, config)
	if err != nil {
		return fmt.Errorf("start sarama producer with address %v failed: %v", k.spec.Backend, err)
	}
	k.setReadyProducer(producer)
	logger.Infof("kafka mqtt filter %s build sarama producer successfully", k.spec.Name())

	go k.watchProducer(producer)
	return nil
}

func (k *Kafka) setReadyProducer(producer sarama.AsyncProducer) {
	k.mu.Lock()
	k.producer = producer
	k.lastErr = ""
	k.mu.Unlock()
}

func (k *Kafka) clearProducer(producer sarama.AsyncProducer) {
	k.mu.Lock()
	if k.producer == producer {
		k.producer = nil
	}
	k.mu.Unlock()
}

func (k *Kafka) getProducer() sarama.AsyncProducer {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.producer
}

func (k *Kafka) setLastErr(err error) {
	k.mu.Lock()
	if err == nil {
		k.lastErr = ""
	} else {
		k.lastErr = producerErrString(err)
	}
	k.mu.Unlock()
}

func producerErrString(err error) string {
	if err == nil {
		return ""
	}
	if producerErr, ok := err.(*sarama.ProducerError); ok && producerErr.Err == nil {
		return "sarama producer failed"
	}
	return err.Error()
}

func (k *Kafka) scheduleReconnect() {
	if !atomic.CompareAndSwapInt32(&k.reconnecting, 0, 1) {
		return
	}
	retryBackoff := producerRetryBackoff
	maxRetryBackoff := producerMaxRetryBackoff

	go func() {
		defer atomic.StoreInt32(&k.reconnecting, 0)
		backoff := retryBackoff
		for {
			if !k.waitReconnect(backoff) {
				return
			}

			err := k.setProducer()
			if err == nil {
				return
			}
			k.setLastErr(err)
			logger.Errorf("%s kafka producer unavailable: %v", k.spec.Name(), err)
			backoff = nextProducerBackoff(backoff, maxRetryBackoff)
		}
	}()
}

func (k *Kafka) waitReconnect(backoff time.Duration) bool {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-k.done:
		return false
	case <-timer.C:
		return true
	}
}

func nextProducerBackoff(backoff, maxBackoff time.Duration) time.Duration {
	backoff *= 2
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}

func (k *Kafka) watchProducer(producer sarama.AsyncProducer) {
	for {
		select {
		case <-k.done:
			if err := producer.Close(); err != nil {
				logger.Errorf("close kafka producer failed: %v", err)
			}
			return
		case err, ok := <-producer.Errors():
			if !ok {
				k.clearProducer(producer)
				k.scheduleReconnect()
				return
			}
			k.setLastErr(err)
			logger.SpanErrorf(nil, "sarama producer failed: %s", producerErrString(err))
		}
	}
}

// Init init Kafka
func (k *Kafka) Init() {
	k.done = make(chan struct{})
	k.setKV()
	if err := k.setProducer(); err != nil {
		k.setLastErr(err)
		logger.Errorf("%s kafka producer unavailable: %v", k.spec.Name(), err)
		k.scheduleReconnect()
	}
}

// Inherit init Kafka based on previous generation
func (k *Kafka) Inherit(previousGeneration filters.Filter) {
	k.Init()
}

// Close close Kafka
func (k *Kafka) Close() {
	if k.done == nil {
		return
	}
	k.closeOnce.Do(func() {
		close(k.done)
	})
}

// Status return status of Kafka
func (k *Kafka) Status() interface{} {
	k.mu.RLock()
	defer k.mu.RUnlock()

	health := k.lastErr
	if health == "" {
		health = "initializing"
		if k.producer != nil {
			health = "ready"
		}
	}

	return &Status{
		Health: health,
		Ready:  k.producer != nil,
	}
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

	producer := k.getProducer()
	if producer == nil {
		if resp, ok := ctx.GetOutputResponse().(*mqttprot.Response); ok && resp != nil {
			resp.SetDrop()
		}
		return resultProducerUnavailable
	}

	select {
	case producer.Input() <- msg:
	case <-k.done:
		if resp, ok := ctx.GetOutputResponse().(*mqttprot.Response); ok && resp != nil {
			resp.SetDrop()
		}
		return resultProducerUnavailable
	}
	return ""
}
