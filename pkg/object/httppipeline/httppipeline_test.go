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
	"github.com/megaease/easegress/pkg/context"
	"testing"
)

type (
	FilterMock struct {
		kind string
	}
)

func (m *FilterMock) Kind() string                                              { return m.kind }
func (m *FilterMock) Close()                                                    {}
func (m *FilterMock) DefaultSpec() interface{}                                  { return &Spec{} }
func (m *FilterMock) Description() string                                       { return "test" }
func (m *FilterMock) Results() []string                                         { return []string{} }
func (m *FilterMock) Handle(ctx context.HTTPContext) (result string)            { return "" }
func (m *FilterMock) Init(filterSpec *FilterSpec)                               {}
func (m *FilterMock) Inherit(filterSpec *FilterSpec, previousGeneration Filter) {}
func (m *FilterMock) Status() interface{}                                       { return nil }

func TestInvalidSpecValidate(t *testing.T) {
	filterRegistry = map[string]Filter{}
	Register(&FilterMock{"mock-filter"})
	Register(&FilterMock{"mock-pipeline"})
	spec := map[string]interface{}{
		"name": "pipeline",
		"kind": "mock-pipeline",
		"flow": []Flow{
			Flow{Filter: "filter-1"}, // no such a filter defined
		},
		"filters": []map[string]interface{}{
			map[string]interface{}{
				"name": "filter-2",
				"kind": "mock-filter",
			},
		},
	}
	_, err := NewFilterSpec(spec, nil)
	if err == nil {
		t.Errorf("spec creation should have failed")
	}
}

func TestSpecValidateDirectedFiltersWithFlow(t *testing.T) {
	filterRegistry = map[string]Filter{}
	Register(&FilterMock{"mock-filter"})
	Register(&FilterMock{"mock-pipeline"})
	spec := map[string]interface{}{
		"name": "pipeline",
		"kind": "mock-pipeline",
		"flow": []Flow{
			Flow{Filter: "filter-1"}, Flow{Filter: "filter-2"},
		},
		"filters": []map[string]interface{}{
			map[string]interface{}{
				"name": "filter-2",
				"kind": "mock-filter",
				// Reference to filter-1 before it's defined.
				// Flow defines the order filters are evaluated, so filter-1 will be available for filter-2.
				"mock-field": "[[filter.filter-1.rsp.body]]",
			},
			map[string]interface{}{
				"name": "filter-1",
				"kind": "mock-filter",
			},
		},
	}
	_, err := NewFilterSpec(spec, nil)
	if err != nil {
		t.Errorf("failed creating valid filter spec %s", err)
	}
}

func TestSpecValidateDirectedFiltersWithoutFlow(t *testing.T) {
	filterRegistry = map[string]Filter{}
	Register(&FilterMock{"mock-filter"})
	Register(&FilterMock{"mock-pipeline"})
	spec := map[string]interface{}{
		"name": "pipeline",
		"kind": "mock-pipeline",
		"filters": []map[string]interface{}{
			map[string]interface{}{
				"name": "filter-2",
				"kind": "mock-filter",
				// Reference to filter-1 before it's defined.
				// There is no Flow so filters are evaluated in the same order as listed here -> this will fail
				"mock-field": "[[filter.filter-1.rsp.body]]",
			},
			map[string]interface{}{
				"name": "filter-1",
				"kind": "mock-filter",
			},
		},
	}
	_, err := NewFilterSpec(spec, nil)
	if err == nil {
		t.Errorf("spec creation should have failed")
	}
}
