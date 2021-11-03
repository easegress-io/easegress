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
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/stretchr/testify/assert"
)

type allProtocolFilter struct {
	mockFilter
}

var _ HTTPFilter = (*allProtocolFilter)(nil)
var _ MQTTFilter = (*allProtocolFilter)(nil)
var _ TCPFilter = (*allProtocolFilter)(nil)

func (f *allProtocolFilter) Kind() string {
	return "allProtocolFilter"
}

func (f *allProtocolFilter) HandleHTTP(ctx context.HTTPContext) *context.HTTPResult {
	return nil
}

func (f *allProtocolFilter) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	return nil
}

func (f *allProtocolFilter) HandleTCP(ctx context.TCPContext) *context.TCPResult {
	return nil
}

func TestRegistry(t *testing.T) {
	a := assert.New(t)

	// test getProtocols
	allFilter := &allProtocolFilter{}
	protocols, err := getProtocols(allFilter)
	a.Nil(err)
	a.Equal(protocols, map[context.Protocol]struct{}{context.HTTP: {}, context.MQTT: {}, context.TCP: {}}, "allProtocolFilter support HTTP, MQTT and TCP")

	nonFilter := &mockFilter{}
	_, err = getProtocols(nonFilter)
	a.NotNil(err, "nonFilter not support any protocol")

	// test GetPipeline
	_, err = GetPipeline("pipeline-no-exist", context.HTTP)
	a.NotNil(err, "GetPipeline not exist pipeline should fail")

	// test store and delete pipeline
	a.Panics(func() { deletePipeline("not-exist", context.HTTP) }, "delete not-exist pipeline should panic")
	a.NotPanics(func() { storePipeline("pipeline-store", context.HTTP, &Pipeline{}) }, "store pipeline should success")
	a.Panics(func() { storePipeline("pipeline-store", context.HTTP, &Pipeline{}) }, "store exist pipeline should panic")
	a.NotPanics(func() { storePipeline("pipeline-store", context.MQTT, &Pipeline{}) }, "store same pipeline name but different protocol name should success")
	a.NotPanics(func() { deletePipeline("pipeline-store", context.HTTP) }, "delete exist pipeline should success")

	Register(&mockFilter{})
	Register(&allProtocolFilter{})
	filters := GetFilterRegistry()
	a.Contains(filters, nonFilter.Kind())
	a.Contains(filters, allFilter.Kind())
}
