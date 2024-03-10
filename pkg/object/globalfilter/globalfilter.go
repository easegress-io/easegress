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

// Package globalfilter provides GlobalFilter.
package globalfilter

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Category is the category of GlobalFilter.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of GlobalFilter.
	Kind = "GlobalFilter"
)

var aliases = []string{"globalfilters"}

func init() {
	supervisor.Register(&GlobalFilter{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  aliases,
	})
}

type (
	// GlobalFilter is a business controller.
	// It provides handler before and after pipeline in HTTPServer.
	GlobalFilter struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		beforePipeline atomic.Value
		afterPipeline  atomic.Value
	}

	// Spec describes the GlobalFilter.
	Spec struct {
		BeforePipeline *pipeline.Spec `json:"beforePipeline,omitempty"`
		AfterPipeline  *pipeline.Spec `json:"afterPipeline,omitempty"`
		Fallthrough    Fallthrough    `json:"fallthrough,omitempty"`
	}

	// Fallthrough describes the fallthrough behavior.
	Fallthrough struct {
		BeforePipeline bool `json:"beforePipeline,omitempty"`
		Pipeline       bool `json:"pipeline,omitempty"`
	}

	// pipelineSpec defines pipeline spec to create an pipeline entity.
	pipelineSpec struct {
		Kind           string `json:"kind,omitempty"`
		Name           string `json:"name,omitempty"`
		*pipeline.Spec `json:",inline"`
	}
)

// Validate validates Spec.
func (s *Spec) Validate() (err error) {
	bothNil := true

	if s.BeforePipeline != nil {
		bothNil = false
		if err := s.BeforePipeline.Validate(); err != nil {
			return fmt.Errorf("before pipeline is invalid: %v", err)
		}
	}

	if s.AfterPipeline != nil {
		bothNil = false
		if err := s.AfterPipeline.Validate(); err != nil {
			return fmt.Errorf("after pipeline is invalid: %v", err)
		}
	}

	if bothNil {
		return fmt.Errorf("both beforePipeline and afterPipeline are nil")
	}

	return nil
}

// Category returns the object category of itself.
func (gf *GlobalFilter) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the unique kind name to represent itself.
func (gf *GlobalFilter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec.
// It must return a pointer to point a struct.
func (gf *GlobalFilter) DefaultSpec() interface{} {
	return &Spec{}
}

// Status returns its runtime status.
func (gf *GlobalFilter) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Init initializes GlobalFilter.
func (gf *GlobalFilter) Init(superSpec *supervisor.Spec) {
	gf.superSpec, gf.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	gf.super = superSpec.Super()
	gf.reload(nil)
}

// Inherit inherits previous generation of GlobalFilter.
func (gf *GlobalFilter) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	gf.superSpec, gf.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	gf.reload(previousGeneration.(*GlobalFilter))
}

func (gf *GlobalFilter) reload(previousGeneration *GlobalFilter) {
	// create and update beforePipeline
	if gf.spec.BeforePipeline != nil {
		var previous *pipeline.Pipeline
		if previousGeneration != nil {
			previous, _ = previousGeneration.beforePipeline.Load().(*pipeline.Pipeline)
		}
		p, err := gf.createPipeline("before", gf.spec.BeforePipeline, previous)
		if err != nil {
			panic(fmt.Errorf("create before pipeline failed: %v", err))
		}
		gf.beforePipeline.Store(p)
	}

	// create and update afterPipeline
	if gf.spec.AfterPipeline != nil {
		var previous *pipeline.Pipeline
		if previousGeneration != nil {
			previous, _ = previousGeneration.afterPipeline.Load().(*pipeline.Pipeline)
		}
		p, err := gf.createPipeline("after", gf.spec.AfterPipeline, previous)
		if err != nil {
			panic(fmt.Errorf("create after pipeline failed: %v", err))
		}
		gf.afterPipeline.Store(p)
	}
}

func (gf *GlobalFilter) createPipeline(name string, spec *pipeline.Spec, previousGeneration *pipeline.Pipeline) (*pipeline.Pipeline, error) {
	jsonSpec := codectool.MustMarshalJSON(&pipelineSpec{
		Kind: pipeline.Kind,
		Name: name,
		Spec: spec,
	})

	fullSpec, err := supervisor.NewSpec(string(jsonSpec))
	if err != nil {
		return nil, err
	}

	// init or update pipeline
	p := &pipeline.Pipeline{}
	if previousGeneration != nil {
		p.Inherit(fullSpec, previousGeneration, nil)
	} else {
		p.Init(fullSpec, nil)
	}

	return p, nil
}

// Handle `beforePipeline` and `afterPipeline` before and after the handler is executed.
func (gf *GlobalFilter) Handle(ctx *context.Context, handler context.Handler) {
	p, ok := handler.(*pipeline.Pipeline)
	if !ok {
		panic("handler is not a pipeline")
	}

	before, _ := gf.beforePipeline.Load().(*pipeline.Pipeline)
	after, _ := gf.afterPipeline.Load().(*pipeline.Pipeline)
	option := pipeline.HandleWithBeforeAfterOption{
		FallthroughBefore:   gf.spec.Fallthrough.BeforePipeline,
		FallthroughPipeline: gf.spec.Fallthrough.Pipeline,
	}
	p.HandleWithBeforeAfter(ctx, before, after, option)
}

// Close closes GlobalFilter itself.
func (gf *GlobalFilter) Close() {
}
