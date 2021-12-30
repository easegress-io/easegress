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

package trafficcontroller

import (
	"fmt"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	logger.InitNop()
	supervisor.Register(&MockPipeline{})
	supervisor.Register(&MockServer{})
}

// MockTrafficObject used to create MockServer and MockPipeline
type MockTrafficObject struct {
	superSpec *supervisor.Spec
	spec      *MockSpec
	mapper    protocol.MuxMapper
}
type MockSpec struct {
	Protocol context.Protocol `yaml:"protocol" jsonschema:"required"`
	Pipeline string           `yaml:"pipeline" jsonschema:"required"`
	Tag      string           `yaml:"tag" jsonschema:"required"`
}

func (obj *MockTrafficObject) DefaultSpec() interface{} { return &MockSpec{} }

func (obj *MockTrafficObject) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	obj.superSpec = superSpec
	obj.spec = superSpec.ObjectSpec().(*MockSpec)
	obj.mapper = muxMapper
}

func (obj *MockTrafficObject) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	obj.Close()
	obj.Init(superSpec, muxMapper)
}

func (obj *MockTrafficObject) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: obj.spec,
	}
}

func (obj *MockTrafficObject) Close() {}

// MockPipeline is a mock pipeline used for test
type MockPipeline struct {
	MockTrafficObject
}

const MockPipelineKind = "MockPipeline"

var _ supervisor.Pipeline = (*MockPipeline)(nil)

func (s *MockPipeline) Category() supervisor.ObjectCategory {
	return supervisor.CategoryPipeline
}

func (p *MockPipeline) Kind() string {
	return MockPipelineKind
}

func (p *MockPipeline) Handle(ctx context.Context) string { return "" }

// MockServer is a server used for test
type MockServer struct {
	MockTrafficObject
}

const MockServerKind = "MockServer"

var _ supervisor.TrafficGate = (*MockServer)(nil)

func (s *MockServer) Category() supervisor.ObjectCategory {
	return supervisor.CategoryTrafficGate
}

func (s *MockServer) Kind() string {
	return MockServerKind
}

func (s *MockServer) CheckPipeline() error {
	_, ok := s.mapper.GetHandler(s.spec.Protocol, s.spec.Pipeline)
	if !ok {
		return fmt.Errorf("server %s get pipeline %s failed", s.superSpec.Name(), s.spec.Pipeline)
	}
	return nil
}

func getTrafficController(t *testing.T) *TrafficController {
	yamlStr := `
name: traffic
kind: TrafficController
`
	super := supervisor.NewDefaultMock()
	spec, err := super.NewSpec(yamlStr)
	require.Nil(t, err)
	tc := &TrafficController{}
	tc.Init(spec)
	return tc
}

func getSuperSpec(t *testing.T, name, kind, protocol, pipeline, tag string) *supervisor.Spec {
	templateStr := `
name: %s
kind: %s
protocol: %s
pipeline: %s
tag: %s
`
	yamlStr := fmt.Sprintf(templateStr, name, kind, protocol, pipeline, tag)
	super := supervisor.NewDefaultMock()
	spec, err := super.NewSpec(yamlStr)
	require.Nil(t, err)
	return spec
}

func checkSpec(t *testing.T, superSpec *supervisor.Spec, spec *MockSpec, name, kind string, protocol context.Protocol, pipeline, tag, msg string) {
	assert := assert.New(t)
	assert.Equal(name, superSpec.Name(), msg)
	assert.Equal(kind, superSpec.Kind(), msg)
	assert.Equal(protocol, spec.Protocol, msg)
	assert.Equal(pipeline, spec.Pipeline, msg)
	assert.Equal(tag, spec.Tag, msg)
}

func TestTrafficController(t *testing.T) {
	assert := assert.New(t)
	tc := getTrafficController(t)
	assert.Equal(supervisor.ObjectCategory(Category), tc.Category())
	assert.Equal(Kind, tc.Kind())
	assert.Equal(&Spec{}, tc.DefaultSpec())

	// check inherit
	yamlStr := `
name: traffic
kind: TrafficController
`
	super := supervisor.NewDefaultMock()
	spec, err := super.NewSpec(yamlStr)
	require.Nil(t, err)
	tc2 := &TrafficController{}
	tc2.Inherit(spec, tc)
	tc2.Close()
}

func TestServer(t *testing.T) {
	assert := assert.New(t)
	tc := getTrafficController(t)
	defer tc.Close()

	ns := "namespace"
	testFn := func(name, kind, protocol, pipeline, tag string, specFn func(string, context.Protocol, *supervisor.Spec) (*supervisor.ObjectEntity, error)) {
		msg := fmt.Sprintf("test case: %s/%s/%s/%s/%s", name, kind, protocol, pipeline, tag)
		spec := getSuperSpec(t, name, kind, protocol, pipeline, tag)
		entity, err := specFn(ns, context.HTTP, spec)
		assert.Nil(err, msg)
		server, ok := entity.Instance().(*MockServer)
		assert.True(ok, msg)
		checkSpec(t, server.superSpec, server.spec, name, kind, context.Protocol(protocol), pipeline, tag, msg)
	}
	// check CreateServerForSpec, create server1
	testFn("server1", MockServerKind, string(context.HTTP), "pipeline1", "version1", tc.CreateServerForSpec)

	// check GetServer, get server1
	entity, ok := tc.GetServer(ns, context.HTTP, "server1")
	assert.True(ok)
	server, ok := entity.Instance().(*MockServer)
	assert.True(ok)
	checkSpec(t, server.superSpec, server.spec, "server1", MockServerKind, context.HTTP, "pipeline1", "version1", "")

	// check UpdateServerForSpec, update server1
	testFn("server1", MockServerKind, string(context.HTTP), "pipeline2", "version2", tc.UpdateServerForSpec)

	// check ApplyServerForSpec, apply server1, apply server2
	testFn("server1", MockServerKind, string(context.HTTP), "pipeline3", "version3", tc.ApplyServerForSpec)
	testFn("server2", MockServerKind, string(context.HTTP), "pipeline4", "version4", tc.ApplyServerForSpec)

	// check DeleteServer, delete server1
	err := tc.DeleteServer(ns, context.HTTP, "server1")
	assert.Nil(err)
	_, ok = tc.GetServer(ns, context.HTTP, "server1")
	assert.False(ok)

	// check ListServers, list only contain server2
	entities := tc.ListServers(ns, context.HTTP)
	require.Equal(t, 1, len(entities))
	entity = entities[0]
	server, ok = entity.Instance().(*MockServer)
	assert.True(ok)
	checkSpec(t, server.superSpec, server.spec, "server2", MockServerKind, context.HTTP, "pipeline4", "version4", "")

	// check walkFn
	allStatus := []*MockSpec{}
	walkFn := func(entity *supervisor.ObjectEntity) bool {
		server, ok = entity.Instance().(*MockServer)
		assert.True(ok)
		status, ok := server.Status().ObjectStatus.(*MockSpec)
		assert.True(ok)
		allStatus = append(allStatus, status)
		return true
	}
	tc.WalkServers(ns, context.HTTP, walkFn)
	assert.Equal(1, len(allStatus))
}

func TestPipeline(t *testing.T) {
	assert := assert.New(t)
	tc := getTrafficController(t)
	defer tc.Close()

	ns := "namespace"
	testFn := func(name, kind, protocol, pipeline, tag string, specFn func(string, context.Protocol, *supervisor.Spec) (*supervisor.ObjectEntity, error)) {
		msg := fmt.Sprintf("test case: %s/%s/%s/%s/%s", name, kind, protocol, pipeline, tag)
		spec := getSuperSpec(t, name, kind, protocol, pipeline, tag)
		entity, err := specFn(ns, context.HTTP, spec)
		assert.Nil(err, msg)
		p, ok := entity.Instance().(*MockPipeline)
		assert.True(ok, msg)
		checkSpec(t, p.superSpec, p.spec, name, kind, context.Protocol(protocol), pipeline, tag, msg)
	}

	// check CreatePipelineForSpec, create pipeline1
	testFn("pipeline1", MockPipelineKind, string(context.HTTP), "", "version1", tc.CreatePipelineForSpec)

	// check GetPipeline, get pipeline1
	entity, ok := tc.GetPipeline(ns, context.HTTP, "pipeline1")
	assert.True(ok)
	pipeline, ok := entity.Instance().(*MockPipeline)
	assert.True(ok)
	checkSpec(t, pipeline.superSpec, pipeline.spec, "pipeline1", MockPipelineKind, context.HTTP, "", "version1", "")

	// check UpdatePipelineForSpec, update pipeline1
	testFn("pipeline1", MockPipelineKind, string(context.HTTP), "", "version2", tc.UpdatePipelineForSpec)

	// check ApplyPipelineForSpec, apply pipeline1, apply pipeline2
	testFn("pipeline1", MockPipelineKind, string(context.HTTP), "", "version3", tc.ApplyPipelineForSpec)
	testFn("pipeline2", MockPipelineKind, string(context.HTTP), "", "version4", tc.ApplyPipelineForSpec)

	// check DeleteServer, delete pipeline1
	err := tc.DeletePipeline(ns, context.HTTP, "pipeline1")
	assert.Nil(err)
	_, ok = tc.GetPipeline(ns, context.HTTP, "pipeline1")
	assert.False(ok)

	// check ListPipelines, list only contain pipelin2
	entities := tc.ListPipelines(ns, context.HTTP)
	require.Equal(t, 1, len(entities))
	entity = entities[0]
	pipeline, ok = entity.Instance().(*MockPipeline)
	assert.True(ok)
	checkSpec(t, pipeline.superSpec, pipeline.spec, "pipeline2", MockPipelineKind, context.HTTP, "", "version4", "")

	// check walkFn
	allStatus := []*MockSpec{}
	walkFn := func(entity *supervisor.ObjectEntity) bool {
		pipeline, ok := entity.Instance().(*MockPipeline)
		assert.True(ok)
		status, ok := pipeline.Status().ObjectStatus.(*MockSpec)
		assert.True(ok)
		allStatus = append(allStatus, status)
		return true
	}
	tc.WalkPipelines(ns, context.HTTP, walkFn)
	assert.Equal(1, len(allStatus))
}
