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
	"reflect"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

func CreateMockedFilter(kind string) filters.Filter {
	return &MockedFilter{kind, []string{}}
}

type (
	MockedFilter struct {
		kind    string
		results []string
	}

	MockedSpec struct {
		filters.BaseSpec `yaml:",inline"`
	}
)

func (m *MockedFilter) Name() string                                                 { return "mock1" }
func (m *MockedFilter) Kind() string                                                 { return m.kind }
func (m *MockedFilter) Close()                                                       {}
func (m *MockedFilter) DefaultSpec() filters.Spec                                    { return &MockedSpec{} }
func (m *MockedFilter) Description() string                                          { return "test" }
func (m *MockedFilter) Results() []string                                            { return m.results }
func (m *MockedFilter) Handle(ctx context.HTTPContext) (result string)               { return "" }
func (m *MockedFilter) Init(spec filters.Spec)                                       {}
func (m *MockedFilter) Inherit(spec filters.Spec, previousGeneration filters.Filter) {}
func (m *MockedFilter) Status() interface{}                                          { return nil }

func cleanup() {
	filters.ResetRegistry()
}

func TestSpecValidate(t *testing.T) {
	cleanup()
	t.Run("spec missing flow", func(t *testing.T) {
		filters.Register(CreateMockedFilter("mock-filter"))
		filters.Register(CreateMockedFilter("mock-pipeline"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"flow": []FlowNode{
				{Filter: "filter-1"}, // no such a filter defined
			},
			"filters": []map[string]interface{}{
				{
					"name": "filter-2",
					"kind": "mock-filter",
				},
			},
		}
		_, err := filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})
	cleanup()
	t.Run("ordered filters with flow", func(t *testing.T) {
		filters.Register(CreateMockedFilter("mock-filter"))
		filters.Register(CreateMockedFilter("mock-pipeline"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"flow": []FlowNode{
				{Filter: "filter-1"}, {Filter: "filter-2"},
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
		_, err := filters.NewSpec(nil, "", spec)
		if err != nil {
			t.Errorf("failed creating valid filter spec %s", err)
		}
	})
	cleanup()
	t.Run("ordered filters without flow", func(t *testing.T) {
		filters.Register(CreateMockedFilter("mock-filter"))
		filters.Register(CreateMockedFilter("mock-pipeline"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
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
		_, err := filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})
	cleanup()
	t.Run("invalid spec", func(t *testing.T) {
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
		}
		_, err := filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		filters.Register(CreateMockedFilter("mock-pipeline"))
		spec = map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"filters": []map[string]interface{}{
				{
					"name": "filter-1",
					"kind": "mock-filter", // missing this
				},
			},
		}
		_, err = filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		spec = map[string]interface{}{"name": "pipeline"}
		_, err = filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})
	cleanup()
	t.Run("duplicate filter", func(t *testing.T) {
		filters.Register(CreateMockedFilter("mock-pipeline"))
		filters.Register(CreateMockedFilter("mock-filter"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"filters": []map[string]interface{}{
				{"name": "filter-1", "kind": "mock-filter"},
				{"name": "filter-1", "kind": "mock-filter"},
			},
		}
		_, err := filters.NewSpec(nil, "", spec)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
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
		filters.Register(CreateMockedFilter("mock-filter"))
		filters.Register(CreateMockedFilter("mock-filter"))
	})
	cleanup()
	t.Run("empty kind", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		filters.Register(CreateMockedFilter(""))
	})
	cleanup()
	t.Run("repeated results", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		results := []string{"res1", "res2", "res3", "res1"}
		filters.Register(&MockedFilter{"filter", results})
	})
	cleanup()
	t.Run("export registry", func(t *testing.T) {
		filters.Register(CreateMockedFilter("mock-pipeline-2"))
		filters := filters.Registry()
		if len(filters) != 1 {
			t.Errorf("couldn't get the filter")
		}
	})
	cleanup()
}

func TestHttpipeline(t *testing.T) {
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
	logger.InitNop()
	filters.Register(CreateMockedFilter("Proxy"))
	filters.Register(CreateMockedFilter("Pipeline"))
	t.Run("missing filter results", func(t *testing.T) {
		filters.Register(CreateMockedFilter("Validator"))
		filters.Register(CreateMockedFilter("RequestAdaptor"))
		_, err := supervisor.NewSpec(superSpecYaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		filters.Unregister("Validator")
		filters.Unregister("RequestAdaptor")
	})
	filters.Register(&MockedFilter{"Validator", []string{"invalid", "END"}})
	filters.Register(&MockedFilter{"RequestAdaptor", []string{"specialCase"}})
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	httpPipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil}
	httpPipeline.Init(superSpec, nil)
	httpPipeline.Inherit(superSpec, &httpPipeline, nil)

	ctx := &contexttest.MockedHTTPContext{}
	httpPipeline.Handle(ctx)
	status := httpPipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if httpPipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	httpPipeline.Close()
	cleanup()
}

func TestHttpipelineNoFlow(t *testing.T) {
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
	logger.InitNop()
	filters.Register(CreateMockedFilter("Proxy"))
	filters.Register(CreateMockedFilter("Pipeline"))
	filters.Register(CreateMockedFilter("Validator"))
	filters.Register(CreateMockedFilter("RequestAdaptor"))

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	httpPipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil}
	httpPipeline.Init(superSpec, nil)
	httpPipeline.Inherit(superSpec, &httpPipeline, nil)

	ctx := &contexttest.MockedHTTPContext{}
	httpPipeline.Handle(ctx)
	status := httpPipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if httpPipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	if httpPipeline.getFilter("proxy") == nil {
		t.Errorf("should have filter")
	}
	httpPipeline.Close()
	cleanup()
}
