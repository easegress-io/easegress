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

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func init() {
	Register(&MockMQTTFilter{})
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
	defer p.Close()

	a := assert.New(t)
	a.Equal(p.spec.Name, "pipeline", "wrong name")
	a.Equal(p.spec.Protocol, context.MQTT, "wrong protocol")
	a.Equal(len(p.spec.Flow), 2, "wrong flow length")
	a.Equal(p.spec.Flow[0].Filter, "mqtt-filter", "wrong filter name")
	a.Equal(len(p.runningFilters), 2, "wrong running filters")

	s := p.runningFilters[0].spec
	a.Equal(s.Name(), "mqtt-filter", "wrong filter name")
	a.Equal(s.Kind(), "MockMQTTFilter", "wrong filter kind")
	a.Equal(s.Pipeline(), "pipeline", "wrong filter pipeline")
	a.Equal(s.Protocol(), context.MQTT, "wrong filter protocol")

	f := p.getRunningFilter("mqtt-filter").filter.(*MockMQTTFilter)
	a.Equal(f.spec.UserName, "test", "wrong filter username")
	a.Equal(f.spec.Port, uint16(1234), "wrong filter port")
	a.Equal(f.spec.BackendType, "Kafka", "wrong filter BackendType")

	f = p.getRunningFilter("mqtt-filter2").filter.(*MockMQTTFilter)
	a.Equal(f.spec.UserName, "", "wrong filter username")
	a.Equal(f.spec.Port, uint16(0), "wrong filter port")
	a.Equal(f.spec.BackendType, "", "wrong filter BackendType")

	pipeline, err := GetPipeline("pipeline", context.MQTT)
	a.Nil(err, "get pipeline failed")
	a.Equal(pipeline, p, "get wrong pipeline")
}

func TestHandleMQTT(t *testing.T) {
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
      backendType: Kafka`
	p := getPipeline(yamlStr, t)
	defer p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			c := &mockMQTTClient{cid: strconv.Itoa(i)}
			ctx := context.NewMQTTContext(stdcontext.Background(), c, nil)
			p.HandleMQTT(ctx)
			wg.Done()
		}(i)
	}
	wg.Wait()
	f := p.getRunningFilter("mqtt-filter").filter.(*MockMQTTFilter)
	assert.Equal(t, len(f.clientCount()), 1000, "wrong client count")
}
