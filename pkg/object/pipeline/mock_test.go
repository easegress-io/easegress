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
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestMockFilter(t *testing.T) {
	a := assert.New(t)

	// mockFilter do nothing, used for create other filters
	mock := &mockFilter{}
	a.Equal(mock.Kind(), "MockFilter")
	a.Equal(mock.DefaultSpec(), &mockSpec{})
	a.Equal(mock.Description(), "mock filter")
	a.Nil(mock.Results())
	a.Nil(mock.Status())
	a.Nil(mock.APIs())
	mock.Init(nil)
	newMock := &mockFilter{}
	newMock.Inherit(nil, mock)
	newMock.Close()
}

func TestMockFilterSpec(t *testing.T) {
	a := assert.New(t)

	super := supervisor.NewDefaultMock()
	rawSpec := map[string]interface{}{"name": "testMock"}
	yamlConfig := "fake yaml config string"
	meta := &FilterMetaSpec{
		Name:     "testMock",
		Kind:     "test",
		Pipeline: "pipeline-demo",
		Protocol: context.TCP,
	}
	filterSpec := &MockMQTTSpec{}
	rootFilter := &MockMQTTFilter{}
	mockSpec := MockFilterSpec(super, rawSpec, yamlConfig, meta, filterSpec, rootFilter)
	a.Equal(mockSpec.Super(), super)
	a.Equal(mockSpec.RawSpec(), rawSpec)
	a.Equal(mockSpec.Kind(), meta.Kind)
	a.Equal(mockSpec.Name(), meta.Name)
	a.Equal(mockSpec.Pipeline(), meta.Pipeline)
	a.Equal(mockSpec.Protocol(), meta.Protocol)
	a.Equal(mockSpec.FilterSpec(), filterSpec)
	a.Equal(mockSpec.RootFilter(), rootFilter)
}
