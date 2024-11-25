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

// Package kafka implements a kafka proxy for HTTP requests.
package kafka

import (
	"fmt"
	"io"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Kind is the kind of Kafka
	Kind = "Kafka"

	resultParseErr = "parseErr"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Kafka is a kafka proxy for HTTP requests",
	Results:     []string{resultParseErr},
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
	// Kafka is a kafka proxy for HTTP requests.
	Kafka struct {
		spec          *Spec
		asyncProducer sarama.AsyncProducer
		syncProcuder  sarama.SyncProducer

		headerTopic string
		headerKey   string
		done        chan struct{}
	}

	// Err is the error of Kafka
	Err struct {
		Err  string `json:"err"`
		Code int    `json:"code"`
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

func (k *Kafka) setHeader() {
	if k.spec.Topic.Dynamic != nil {
		k.headerTopic = http.CanonicalHeaderKey(k.spec.Topic.Dynamic.Header)
		if k.headerTopic == "" {
			panic("empty header topic")
		}
	}
	if k.spec.Key.Dynamic != nil {
		k.headerKey = http.CanonicalHeaderKey(k.spec.Key.Dynamic.Header)
		if k.headerKey == "" {
			panic("empty header key")
		}
	}
}

// Init init Kafka
func (k *Kafka) Init() {
	spec := k.spec
	k.done = make(chan struct{})
	k.setHeader()

	config := sarama.NewConfig()
	config.ClientID = spec.Name()
	config.Version = sarama.V1_0_0_0
	if spec.Sync {
		config.Producer.Return.Successes = true
		producer, err := sarama.NewSyncProducer(k.spec.Backend, config)
		if err != nil {
			panic(fmt.Errorf("start sarama sync producer with address %v failed: %v", spec.Backend, err))
		}
		k.syncProcuder = producer
	} else {
		producer, err := sarama.NewAsyncProducer(k.spec.Backend, config)
		if err != nil {
			panic(fmt.Errorf("start sarama async producer with address %v failed: %v", k.spec.Backend, err))
		}

		k.asyncProducer = producer
		go k.checkProduceError()
	}
}

func (k *Kafka) checkProduceError() {
	for {
		select {
		case <-k.done:
			err := k.asyncProducer.Close()
			if err != nil {
				logger.Errorf("close kafka producer failed: %v", err)
			}
			return
		case err, ok := <-k.asyncProducer.Errors():
			if !ok {
				return
			}
			logger.Errorf("sarama producer failed: %v", err)
		}
	}
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

func (k *Kafka) getTopic(req *httpprot.Request) string {
	if k.headerTopic == "" {
		return k.spec.Topic.Default
	}
	topic := req.Std().Header.Get(k.headerTopic)
	if topic == "" {
		return k.spec.Topic.Default
	}
	return topic
}

func (k *Kafka) getKey(req *httpprot.Request) string {
	if k.headerKey == "" {
		return k.spec.Key.Default
	}
	key := req.Std().Header.Get(k.headerKey)
	if key == "" {
		return k.spec.Key.Default
	}
	return key
}

// Handle handles the context.
func (k *Kafka) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	topic := k.getTopic(req)
	key := k.getKey(req)

	body, err := io.ReadAll(req.GetPayload())
	if err != nil {
		return resultParseErr
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(body),
	}
	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}

	if k.spec.Sync {
		_, _, err = k.syncProcuder.SendMessage(msg)
		if err != nil {
			logger.Errorf("send message to kafka failed: %v", err)
			setErrResponse(ctx, err)
		} else {
			setSuccessResponse(ctx)
		}
		return ""
	}

	k.asyncProducer.Input() <- msg
	return ""
}

func setSuccessResponse(ctx *context.Context) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	resp.SetStatusCode(http.StatusOK)
	ctx.SetOutputResponse(resp)
}

func setErrResponse(ctx *context.Context, err error) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	resp.SetStatusCode(http.StatusInternalServerError)
	errMsg := &Err{
		Err:  err.Error(),
		Code: http.StatusInternalServerError,
	}
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
	ctx.SetOutputResponse(resp)
}
