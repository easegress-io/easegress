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

package httppipeline

import (
	"reflect"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

func CreateObjectMock(kind string) Filter {
	return &FilterMock{kind, []string{}}
}

type (
	FilterMock struct {
		kind    string
		results []string
	}
)

func (m *FilterMock) Kind() string                                              { return m.kind }
func (m *FilterMock) Close()                                                    {}
func (m *FilterMock) DefaultSpec() interface{}                                  { return &Spec{} }
func (m *FilterMock) Description() string                                       { return "test" }
func (m *FilterMock) Results() []string                                         { return m.results }
func (m *FilterMock) Handle(ctx context.HTTPContext) (result string)            { return "" }
func (m *FilterMock) Init(filterSpec *FilterSpec)                               {}
func (m *FilterMock) Inherit(filterSpec *FilterSpec, previousGeneration Filter) {}
func (m *FilterMock) Status() interface{}                                       { return nil }

func cleanup() {
	filterRegistry = map[string]Filter{}
}

func TestSpecValidate(t *testing.T) {
	cleanup()
	t.Run("spec missing flow", func(t *testing.T) {
		Register(CreateObjectMock("mock-filter"))
		Register(CreateObjectMock("mock-pipeline"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"flow": []Flow{
				{Filter: "filter-1"}, // no such a filter defined
			},
			"filters": []map[string]interface{}{
				{
					"name": "filter-2",
					"kind": "mock-filter",
				},
			},
		}
		_, err := NewFilterSpec(spec, nil)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})
	cleanup()
	t.Run("ordered filters with flow", func(t *testing.T) {
		Register(CreateObjectMock("mock-filter"))
		Register(CreateObjectMock("mock-pipeline"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"flow": []Flow{
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
		_, err := NewFilterSpec(spec, nil)
		if err != nil {
			t.Errorf("failed creating valid filter spec %s", err)
		}
	})
	cleanup()
	t.Run("ordered filters without flow", func(t *testing.T) {
		Register(CreateObjectMock("mock-filter"))
		Register(CreateObjectMock("mock-pipeline"))
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
		_, err := NewFilterSpec(spec, nil)
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
		_, err := NewFilterSpec(spec, nil)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		Register(CreateObjectMock("mock-pipeline"))
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
		_, err = NewFilterSpec(spec, nil)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		spec = map[string]interface{}{"name": "pipeline"}
		_, err = NewFilterSpec(spec, nil)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})
	cleanup()
	t.Run("duplicate filter", func(t *testing.T) {
		Register(CreateObjectMock("mock-pipeline"))
		Register(CreateObjectMock("mock-filter"))
		spec := map[string]interface{}{
			"name": "pipeline",
			"kind": "mock-pipeline",
			"filters": []map[string]interface{}{
				{"name": "filter-1", "kind": "mock-filter"},
				{"name": "filter-1", "kind": "mock-filter"},
			},
		}
		_, err := NewFilterSpec(spec, nil)
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
		Register(CreateObjectMock("mock-filter"))
		Register(CreateObjectMock("mock-filter"))
	})
	cleanup()
	t.Run("empty kind", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		Register(CreateObjectMock(""))
	})
	cleanup()
	t.Run("repeated results", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		results := []string{"res1", "res2", "res3", "res1"}
		Register(&FilterMock{"filter", results})
	})
	cleanup()
	t.Run("export registry", func(t *testing.T) {
		Register(CreateObjectMock("mock-pipeline-2"))
		filters := GetFilterRegistry()
		if len(filters) != 1 {
			t.Errorf("couldn't get the filter")
		}
	})
	cleanup()
}

func TestHttpipeline(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: HTTPPipeline
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
	Register(CreateObjectMock("Proxy"))
	Register(CreateObjectMock("HTTPPipeline"))
	t.Run("missing filter results", func(t *testing.T) {
		Register(CreateObjectMock("Validator"))
		Register(CreateObjectMock("RequestAdaptor"))
		_, err := supervisor.NewSpec(superSpecYaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		delete(filterRegistry, "Validator")
		delete(filterRegistry, "RequestAdaptor")
	})
	Register(&FilterMock{"Validator", []string{"invalid", "END"}})
	Register(&FilterMock{"RequestAdaptor", []string{"specialCase"}})
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	httpPipeline := HTTPPipeline{nil, nil, nil, []*runningFilter{}, nil}
	httpPipeline.Init(superSpec, nil)
	httpPipeline.Inherit(superSpec, &httpPipeline, nil)

	t.Run("test getNextFilterIndex", func(t *testing.T) {
		if ind, end := httpPipeline.getNextFilterIndex(0, ""); ind != 1 && end != false {
			t.Errorf("next index should be 1, was %d", ind)
		}
		if ind, end := httpPipeline.getNextFilterIndex(0, "invalid"); ind != 3 && end != true {
			t.Errorf("next index should be 3, was %d", ind)
		}
		if ind, end := httpPipeline.getNextFilterIndex(0, "unknown"); ind != -1 && end != false {
			t.Errorf("next index should be -1, was %d", ind)
		}
		if ind, end := httpPipeline.getNextFilterIndex(1, "specialCase"); ind != 2 && end != false {
			t.Errorf("next index should be 2, was %d", ind)
		}
	})

	ctx := &contexttest.MockedHTTPContext{}
	httpPipeline.Handle(ctx)
	status := httpPipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if httpPipeline.getRunningFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	httpPipeline.Close()
	cleanup()
}

func TestHttpipelineNoFlow(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: HTTPPipeline
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
	Register(CreateObjectMock("Proxy"))
	Register(CreateObjectMock("HTTPPipeline"))
	Register(CreateObjectMock("Validator"))
	Register(CreateObjectMock("RequestAdaptor"))

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	httpPipeline := HTTPPipeline{nil, nil, nil, []*runningFilter{}, nil}
	httpPipeline.Init(superSpec, nil)
	httpPipeline.Inherit(superSpec, &httpPipeline, nil)

	ctx := &contexttest.MockedHTTPContext{}
	httpPipeline.Handle(ctx)
	status := httpPipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if httpPipeline.getRunningFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	if httpPipeline.getRunningFilter("proxy") == nil {
		t.Errorf("should have filter")
	}
	httpPipeline.Close()
	cleanup()
}

func TestMockFilterSpec(t *testing.T) {
	meta := &FilterMetaSpec{
		Name:     "name",
		Kind:     "kind",
		Pipeline: "pipeline-demo",
	}
	spec := &FilterSpec{}
	filterSpec := MockFilterSpec(nil, nil, "", meta, spec)
	if filterSpec.Super() != nil {
		t.Errorf("expect nil")
	}
	if filterSpec.Pipeline() != "pipeline-demo" {
		t.Errorf("expect empty string")
	}
	if filterSpec.RawSpec() != nil {
		t.Errorf("expect nil")
	}
	if filterSpec.FilterSpec() != spec {
		t.Errorf("expect spec")
	}
}
