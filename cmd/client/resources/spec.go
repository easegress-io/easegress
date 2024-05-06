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

// Package resources provides the resources utilities for the client.
package resources

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const defaultTableType = "__default__"

// kindToSpec maps the kind to the spec.
var kindToSpec = map[string]func() SpecInfo{
	httpserver.Kind:  func() SpecInfo { return &HTTPServerSpec{} },
	defaultTableType: func() SpecInfo { return &MetaSpec{} },
}

// GetSpecInfo returns the spec of the object, used to generate table.
func GetSpecInfo(kind string, data []byte) (SpecInfo, error) {
	specFn, ok := kindToSpec[kind]
	if !ok {
		specFn = kindToSpec[defaultTableType]
	}
	spec := specFn()
	if err := json.Unmarshal(data, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// Table is the table type.
type Table [][]string

// TableRow is the row type.
type TableRow []string

// WithNamespace adds the namespace to the row.
func (row TableRow) WithNamespace(namespace string) TableRow {
	return append(row, namespace)
}

// SpecInfo is the interface for the object specs to get information used by table.
type SpecInfo interface {
	GetName() string
	GetKind() string
	GetAge() string
	TableHeader() (string, TableRow)
	TableRow() (string, TableRow)
}

// MetaSpec is the meta spec for most objects.
type MetaSpec struct {
	supervisor.MetaSpec `json:",inline"`
}

var _ SpecInfo = &MetaSpec{}

// GetName returns the name of the object.
func (m *MetaSpec) GetName() string {
	return m.Name
}

// GetKind returns the kind of the object.
func (m *MetaSpec) GetKind() string {
	return m.Kind
}

// GetAge returns the age of the object.
func (m *MetaSpec) GetAge() string {
	createdAt, err := time.Parse(time.RFC3339, m.CreatedAt)
	if err != nil {
		return "unknown"
	}
	return general.DurationMostSignificantUnit(time.Since(createdAt))
}

// TableHeader returns the table header.
func (m *MetaSpec) TableHeader() (string, TableRow) {
	return defaultTableType, TableRow{"NAME", "KIND", "AGE"}
}

// TableRow returns the table row.
func (m *MetaSpec) TableRow() (string, TableRow) {
	return defaultTableType, TableRow{m.Name, m.Kind, m.GetAge()}
}

// HTTPServerSpec is the spec for HTTPServer.
type HTTPServerSpec struct {
	MetaSpec        `json:",inline"`
	httpserver.Spec `json:",inline"`
}

var _ SpecInfo = &HTTPServerSpec{}

// TableHeader returns the table header.
func (h *HTTPServerSpec) TableHeader() (string, TableRow) {
	return httpserver.Kind, TableRow{"NAME", "KIND", "PORT", "HTTPS", "AGE"}
}

// TableRow returns the table row.
func (h *HTTPServerSpec) TableRow() (string, TableRow) {
	return httpserver.Kind, TableRow{h.Name, h.Kind, strconv.Itoa(int(h.Port)), strconv.FormatBool(h.HTTPS), h.GetAge()}
}
