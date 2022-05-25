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
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func init() {
	logger.InitNop()
}

type MockedFilter struct {
	kind  *filters.Kind
	spec  *MockedSpec
	count int
}

type MockedSpec struct {
	filters.BaseSpec `yaml:",inline"`
}

type MockedStatus struct {
	Count int
}

func (m *MockedFilter) Name() string                              { return m.spec.Name() }
func (m *MockedFilter) Kind() *filters.Kind                       { return m.kind }
func (m *MockedFilter) Spec() filters.Spec                        { return nil }
func (m *MockedFilter) Close()                                    {}
func (m *MockedFilter) Init()                                     {}
func (m *MockedFilter) Inherit(previousGeneration filters.Filter) {}

func (m *MockedFilter) HeaderKV() (string, string) {
	return fmt.Sprintf("X-Mock-%s", m.spec.Name()), m.spec.Name()
}

func (m *MockedFilter) Status() interface{} {
	return &MockedStatus{m.count}
}

func (m *MockedFilter) Handle(ctx *context.Context) (result string) {
	m.count++
	req := ctx.Request()
	if r, ok := req.(*httpprot.Request); ok {
		k, v := m.HeaderKV()
		r.HTTPHeader().Set(k, v)
	}
	return ""
}

func MockFilterKind(kind string, results []string) *filters.Kind {
	k := &filters.Kind{
		Name:        kind,
		Description: kind,
		Results:     results,
		DefaultSpec: func() filters.Spec {
			return &MockedSpec{}
		},
	}
	k.CreateInstance = func(spec filters.Spec) filters.Filter {
		return &MockedFilter{
			kind: k,
			spec: spec.(*MockedSpec),
		}
	}
	return k
}

func cleanup() {
	filters.ResetRegistry()
}

func TestSpecValidate(t *testing.T) {
	cleanup()
	t.Run("spec missing flow", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-filter", nil))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "Pipeline",
			"flow": []FlowNode{
				{FilterName: "filter-1"}, // no such a filter defined
			},
			"filters": []map[string]interface{}{
				{
					"name": "filter-2",
					"kind": "mock-filter",
				},
			},
		}
		superSpecYaml, err := yaml.Marshal(spec)
		assert.Nil(t, err)
		_, err = supervisor.NewSpec(string(superSpecYaml))
		assert.NotNil(t, err, "filter-1 not found")
	})
	cleanup()
	t.Run("ordered filters with flow", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-filter", nil))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "Pipeline",
			"flow": []FlowNode{
				{FilterName: "filter-1"}, {FilterName: "filter-2"},
			},
			"filters": []map[string]interface{}{
				{
					"name": "filter-2",
					"kind": "mock-filter",
					// Reference to filter-1 before it's defined.
					// Flow defines the order filters are evaluated, so filter-1 will be available for filter-2.
					"mock-field": "[[filter.filter-1.rsp.body]]",
				},
				{
					"name": "filter-1",
					"kind": "mock-filter",
				},
			},
		}
		superSpecYaml, err := yaml.Marshal(spec)
		assert.Nil(t, err)
		_, err = supervisor.NewSpec(string(superSpecYaml))
		assert.Nil(t, err, "valid spec")
	})
	cleanup()
	t.Run("ordered filters without flow", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-filter", nil))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "Pipeline",
			"filters": []map[string]interface{}{
				{
					"name": "filter-2",
					"kind": "mock-filter",
					// Reference to filter-1 before it's defined.
					// There is no Flow so filters are evaluated in the same order as listed here -> this will fail
					"mock-field": "[[filter.filter-1.rsp.body]]",
				},
				{
					"name": "filter-1",
					"kind": "mock-filter",
				},
			},
		}
		superSpecYaml, err := yaml.Marshal(spec)
		assert.Nil(t, err)
		_, err = supervisor.NewSpec(string(superSpecYaml))
		assert.Nil(t, err, "valid spec")
	})
	cleanup()
	t.Run("duplicate filter", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-filter", nil))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "Pipeline",
			"filters": []map[string]interface{}{
				{"name": "filter-1", "kind": "mock-filter"},
				{"name": "filter-1", "kind": "mock-filter"},
			},
		}
		superSpecYaml, err := yaml.Marshal(spec)
		assert.Nil(t, err)
		_, err = supervisor.NewSpec(string(superSpecYaml))
		assert.NotNil(t, err, "invalid spec")
	})
	cleanup()
}

func TestRegistry(t *testing.T) {
	cleanup()
	t.Run("duplicate filter name", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		filters.Register(MockFilterKind("mock-filter", nil))
		filters.Register(MockFilterKind("mock-filter", nil))
	})
	cleanup()
	t.Run("empty kind", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		filters.Register(MockFilterKind("", nil))
	})
	cleanup()
	t.Run("repeated results", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		results := []string{"res1", "res2", "res3", "res1"}
		filters.Register(MockFilterKind("filter", results))
	})
	cleanup()
	t.Run("export registry", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-pipeline-2", nil))
		count := 0
		filters.WalkKind(func(k *filters.Kind) bool {
			count++
			return true
		})
		if count != 1 {
			t.Errorf("couldn't get the filter")
		}
	})
	cleanup()
}

func TestHTTPPipeline(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
flow:
  - filter: validator
    jumpIf: { invalid: END }
  - filter: requestAdaptor
    jumpIf: { specialCase: proxy }
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
        - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin
  - name: requestAdaptor
    kind: RequestAdaptor
    header:
      set:
        X-Adapt-Key: goodplan
  - name: validator
    kind: Validator
    headers:
      Content-Type:
        values:
        - application/json
`
	filters.Register(MockFilterKind("Proxy", nil))
	filters.Register(MockFilterKind("Pipeline", nil))
	t.Run("missing filter results", func(t *testing.T) {
		filters.Register(MockFilterKind("Validator", nil))
		filters.Register(MockFilterKind("RequestAdaptor", nil))
		_, err := supervisor.NewSpec(superSpecYaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		filters.Unregister("Validator")
		filters.Unregister("RequestAdaptor")
	})
	filters.Register(MockFilterKind("Validator", []string{"invalid", "END"}))
	filters.Register(MockFilterKind("RequestAdaptor", []string{"specialCase"}))
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	pipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil, nil}
	pipeline.Init(superSpec, nil)
	pipeline.Inherit(superSpec, &pipeline, nil)

	status := pipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if pipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	pipeline.Close()
	cleanup()
}

func TestHTTPPipelineNoFlow(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
filters:
  - name: validator
    kind: Validator
    headers:
      Content-Type:
        values:
        - application/json
  - name: requestAdaptor
    kind: RequestAdaptor
    header:
      set:
        X-Adapt-Key: goodplan
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin
`
	filters.Register(MockFilterKind("Proxy", nil))
	filters.Register(MockFilterKind("Pipeline", nil))
	filters.Register(MockFilterKind("Validator", nil))
	filters.Register(MockFilterKind("RequestAdaptor", nil))

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	pipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil, nil}
	pipeline.Init(superSpec, nil)
	pipeline.Inherit(superSpec, &pipeline, nil)

	status := pipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if pipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	if pipeline.getFilter("proxy") == nil {
		t.Errorf("should have filter")
	}
	pipeline.Close()
	cleanup()
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
flow:
  - filter: filter1
  - filter: filter2 
filters:
  - name: filter1 
    kind: Filter1 
  - name: filter2
    kind: Filter2 
`
	filters.Register(MockFilterKind("Filter1", nil))
	filters.Register(MockFilterKind("Filter2", nil))
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)

	pipeline := &Pipeline{}
	pipeline.Init(superSpec, nil)
	defer pipeline.Close()
	defer cleanup()

	stdReq, err := http.NewRequest(http.MethodGet, "http://localhost:9095", nil)
	assert.Nil(err)
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(err)

	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.InitialRequestID, req)
	ctx.UseRequest(context.InitialRequestID, context.InitialRequestID)

	pipeline.Handle(ctx)

	filter1 := MockGetFilter(pipeline, "filter1").(*MockedFilter)
	k1, v1 := filter1.HeaderKV()
	filter2 := MockGetFilter(pipeline, "filter2").(*MockedFilter)
	k2, v2 := filter2.HeaderKV()
	assert.Equal(v1, stdReq.Header.Get(k1))
	assert.Equal(v2, stdReq.Header.Get(k2))
	fmt.Printf("tags %+v\n", ctx.Tags())

	assert.True(strings.Contains(ctx.Tags(), "filter1"), "current: filter1->filter2")
	assert.True(strings.Contains(ctx.Tags(), "filter2"))

	status := pipeline.Status().ObjectStatus.(*Status)
	assert.Equal(2, len(status.Filters))
	assert.Empty(status.ToMetrics("123"), "no metrics")
}
