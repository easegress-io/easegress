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

package builder

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
)

const (
	// DataBuilderKind is the kind of DataBuilder.
	DataBuilderKind = "DataBuilder"
)

var dataBuilderKind = &filters.Kind{
	Name:        DataBuilderKind,
	Description: "DataBuilder builds and stores data",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &DataBuilderSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &DataBuilder{spec: spec.(*DataBuilderSpec)}
	},
}

func init() {
	filters.Register(dataBuilderKind)
}

type (
	// DataBuilder is filter DataBuilder.
	DataBuilder struct {
		spec *DataBuilderSpec
		Builder
	}

	// DataBuilderSpec is DataBuilder Spec.
	DataBuilderSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`
		DataKey          string `json:"dataKey,omitempty"`
	}
)

// Validate validates the DataBuilder Spec.
func (spec *DataBuilderSpec) Validate() error {
	if spec.DataKey == "" {
		return fmt.Errorf("dataKey must be specified")
	}

	if spec.Template == "" {
		return fmt.Errorf("template must be specified")
	}

	return spec.Spec.Validate()
}

// Name returns the name of the DataBuilder filter instance.
func (db *DataBuilder) Name() string {
	return db.spec.Name()
}

// Kind returns the kind of DataBuilder.
func (db *DataBuilder) Kind() *filters.Kind {
	return dataBuilderKind
}

// Spec returns the spec used by the DataBuilder
func (db *DataBuilder) Spec() filters.Spec {
	return db.spec
}

// Init initializes DataBuilder.
func (db *DataBuilder) Init() {
	db.reload()
}

// Inherit inherits previous generation of DataBuilder.
func (db *DataBuilder) Inherit(previousGeneration filters.Filter) {
	db.Init()
}

func (db *DataBuilder) reload() {
	db.Builder.reload(&db.spec.Spec)
}

// Handle builds request.
func (db *DataBuilder) Handle(ctx *context.Context) (result string) {
	data, err := prepareBuilderData(ctx)
	if err != nil {
		logger.Warnf("prepareBuilderData failed: %v", err)
		return resultBuildErr
	}

	var r interface{}
	if err = db.build(data, &r); err != nil {
		msgFmt := "DataBuilder(%s): failed to build data: %v"
		logger.Warnf(msgFmt, db.Name(), err)
		return resultBuildErr
	}

	ctx.SetData(db.spec.DataKey, r)
	return ""
}
