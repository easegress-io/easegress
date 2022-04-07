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

package pipeline

import (
	stdcontext "context"
	"strconv"
	"sync"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func init() {
	Register(&MockMQTTFilter{})
	logger.InitNop()
}

func getPipeline(yamlStr string, t *testing.T) *Pipeline {
	super := supervisor.NewDefaultMock()
	superSpec, err := super.NewSpec(yamlStr)
	if err != nil {
		t.Errorf("supervisor unmarshal yaml failed, %s", err)
		t.Skip()
	}
	p := &Pipeline{}
	p.Init(superSpec)
	return p
}

func TestPipeline(t *testing.T) {
	assert := assert.New(t)

	yamlStr := `
    name: pipeline
    kind: Pipeline
    protocol: MQTT
    flow:
    - filter: mqtt-filter
    - filter: mqtt-filter2
    filters:
    - name: mqtt-filter
      kind: MockMQTTFilter
      userName: test
      port: 1234
      backendType: Kafka
    - name: mqtt-filter2
      kind: MockMQTTFilter`
	p := getPipeline(yamlStr, t)

	assert.Equal(p.spec.Name, "pipeline", "wrong name")
	assert.Equal(p.spec.Protocol, context.MQTT, "wrong protocol")
	assert.Equal(len(p.spec.Flow), 2, "wrong flow length")
	assert.Equal(p.spec.Flow[0].Filter, "mqtt-filter", "wrong filter name")
	assert.Equal(len(p.runningFilters), 2, "wrong running filters")

	s := p.runningFilters[0].spec
	assert.Equal(s.Name(), "mqtt-filter", "wrong filter name")
	assert.Equal(s.Kind(), "MockMQTTFilter", "wrong filter kind")
	assert.Equal(s.Pipeline(), "pipeline", "wrong filter pipeline")
	assert.Equal(s.Protocol(), context.MQTT, "wrong filter protocol")

	f := p.getRunningFilter("mqtt-filter").filter.(*MockMQTTFilter)
	assert.Equal(f.spec.UserName, "test", "wrong filter username")
	assert.Equal(f.spec.Port, uint16(1234), "wrong filter port")
	assert.Equal(f.spec.BackendType, "Kafka", "wrong filter BackendType")

	f = p.getRunningFilter("mqtt-filter2").filter.(*MockMQTTFilter)
	assert.Equal(f.spec.UserName, "", "wrong filter username")
	assert.Equal(f.spec.Port, uint16(0), "wrong filter port")
	assert.Equal(f.spec.BackendType, "", "wrong filter BackendType")

	pipeline, err := GetPipeline("pipeline", context.MQTT)
	assert.Nil(err, "get pipeline failed")
	assert.Equal(pipeline, p, "get wrong pipeline")

	status := p.Status().ObjectStatus.(*Status)
	assert.Equal(len(status.Filters), 2)

	filter := p.getRunningFilter("not-exist-filter")
	assert.Nil(filter)

	newP := &Pipeline{}
	newP.Inherit(p.superSpec, p)
	newP.Close()
}

func TestHandleMQTT(t *testing.T) {
	assert := assert.New(t)

	yamlStr := `
name: pipeline
kind: Pipeline
protocol: MQTT
flow:
- filter: mqtt-filter
filters:
- name: mqtt-filter
  kind: MockMQTTFilter
  userName: test
  port: 1234
  earlyStop: true
  backendType: Kafka
  keysToStore:
  - mock
  - mqtt
  - filter
  publishBackendClientID: true`
	p := getPipeline(yamlStr, t)
	defer p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			c := &mockMQTTClient{cid: strconv.Itoa(i), userName: strconv.Itoa(i + 1)}
			publish := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			ctx := context.NewMQTTContext(stdcontext.Background(), c, publish)
			assert.Equal(ctx.Client().UserName(), strconv.Itoa(i+1))
			p.HandleMQTT(ctx)
			_, ok := ctx.Client().Load("mock")
			assert.Equal(true, ok)
			_, ok = ctx.Client().Load("mqtt")
			assert.Equal(true, ok)
			_, ok = ctx.Client().Load("filter")
			assert.Equal(true, ok)

			subscribe := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
			subscribe.Topics = []string{strconv.Itoa(i)}
			ctx = context.NewMQTTContext(stdcontext.Background(), c, subscribe)
			p.HandleMQTT(ctx)

			unsubscribe := packets.NewControlPacket(packets.Unsubscribe).(*packets.UnsubscribePacket)
			subscribe.Topics = []string{strconv.Itoa(i)}
			ctx = context.NewMQTTContext(stdcontext.Background(), c, unsubscribe)
			p.HandleMQTT(ctx)

			wg.Done()
		}(i)
	}
	wg.Wait()
	f := p.getRunningFilter("mqtt-filter").filter.(*MockMQTTFilter)
	status := f.Status().(MockMQTTStatus)
	assert.Equal(len(status.ClientCount), 1000, "wrong client count")
	assert.Equal(1000, len(status.Subscribe))
	assert.Equal(1000, len(status.Unsubscribe))

	newP := &Pipeline{}
	newP.spec = &Spec{Protocol: context.HTTP}
	assert.NotPanics(func() { newP.HandleMQTT(nil) }, "handle mqtt will log and return since pipeline protocol is http")

	yamlStr = `
    name: pipeline-no-flow
    kind: Pipeline
    protocol: MQTT
    filters:
    - name: mqtt-filter
      kind: MockMQTTFilter
      userName: test
      port: 1234
      earlyStop: true
      backendType: Kafka`
	assert.NotPanics(func() { getPipeline(yamlStr, t) }, "no flow should work")

	yamlStr = `
    name: pipeline-flow-no-filter
    kind: Pipeline
    protocol: MQTT
    flow:
    - filter: mqtt-filter
    filters:
    - name: http-filter
      kind: MockMQTTFilter
      userName: test
      port: 1234
      earlyStop: true
      backendType: Kafka`
	assert.Panics(func() { getPipeline(yamlStr, t) }, "flow and filter have different name")

	yamlStr = `
    name: pipeline-flow-no-filter
    kind: Pipeline
    protocol: MQTT
    flow:
    - filter: mqtt-filter`
	assert.Panics(func() { getPipeline(yamlStr, t) }, "flow and no filter should panic")
}
