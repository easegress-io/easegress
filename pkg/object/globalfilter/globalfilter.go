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

package globalfilter

import (
	"fmt"
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

const (
	// Category is the category of GlobalFilter.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of GlobalFilter.
	Kind = "GlobalFilter"

	beforePipelineKey = "before"
	afterPipelineKey  = "after"
)

type (
	// GlobalFilter is a business controller
	// provide handler before and after pipeline in HTTPServer
	GlobalFilter struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		// pipelines map[string]*httppipeline.HTTPPipeline
		// only contains two key: before and after
		pipelines *sync.Map
	}

	// Spec describes the GlobalFilter.
	Spec struct {
		BeforePipeline httppipeline.Spec `yaml:"beforePipeline" jsonschema:"omitempty"`
		AfterPipeline  httppipeline.Spec `yaml:"afterPipeline" jsonschema:"omitempty"`
	}

	// pipelineSpec define httppipeline spec to create a httppipeline entity
	pipelineSpec struct {
		Kind              string `yaml:"kind" jsonschema:"omitempty"`
		Name              string `yaml:"name" jsonschema:"omitempty"`
		httppipeline.Spec `yaml:",inline"`
	}
)

func init() {
	supervisor.Register(&GlobalFilter{})
}

// CreateAndUpdateBeforePipelineForSpec ...
func (gf *GlobalFilter) CreateAndUpdateBeforePipelineForSpec(spec *Spec, previousGeneration *httppipeline.HTTPPipeline) error {
	beforePipeline := &pipelineSpec{
		Kind: httppipeline.Kind,
		Name: beforePipelineKey,
		Spec: spec.BeforePipeline,
	}
	return gf.CreateAndUpdatePipeline(beforePipeline, previousGeneration)
}

// CreateAndUpdateAfterPipelineForSpec ...
func (gf *GlobalFilter) CreateAndUpdateAfterPipelineForSpec(spec *Spec, previousGeneration *httppipeline.HTTPPipeline) error {
	afterPipeline := &pipelineSpec{
		Kind: httppipeline.Kind,
		Name: beforePipelineKey,
		Spec: spec.AfterPipeline,
	}
	return gf.CreateAndUpdatePipeline(afterPipeline, previousGeneration)
}

// CreateAndUpdatePipeline create and update globalFilter`s pipelines from pipeline spec
func (gf *GlobalFilter) CreateAndUpdatePipeline(spec *pipelineSpec, previousGeneration *httppipeline.HTTPPipeline) error {
	// init config
	config := yamltool.Marshal(spec)
	specS, err := supervisor.NewSpec(string(config))
	if err != nil {
		return err
	}

	// init or update pipeline
	var pipeline = new(httppipeline.HTTPPipeline)
	if previousGeneration != nil {
		pipeline.Inherit(specS, previousGeneration, nil)
	} else {
		pipeline.Init(specS, nil)
	}
	gf.pipelines.Store(spec.Name, pipeline)
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
	gf.pipelines = &sync.Map{}
	gf.reload(nil)
}

// Inherit inherits previous generation of GlobalFilter.
func (gf *GlobalFilter) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	gf.superSpec, gf.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	gf.reload(previousGeneration.(*GlobalFilter))
}

// BeforeHandle before handler logic for beforePipeline spec
func (gf *GlobalFilter) BeforeHandle(ctx context.HTTPContext) {
	handler, ok := gf.getPipeline(beforePipelineKey)
	if !ok {
		return
	}
	handler.Handle(ctx)
}

// AfterHandle after handler logic for afterPipeline spec
func (gf *GlobalFilter) AfterHandle(ctx context.HTTPContext) {
	handler, ok := gf.getPipeline(afterPipelineKey)
	if !ok {
		return
	}
	handler.Handle(ctx)
}

func (gf *GlobalFilter) getPipeline(key string) (*httppipeline.HTTPPipeline, bool) {
	value, ok := gf.pipelines.Load(key)
	if !ok || value == nil {
		return nil, false
	}
	pipe, ok := value.(*httppipeline.HTTPPipeline)
	return pipe, ok
}

// Close closes itself. It is called by deleting.
// Supervisor won't call Close for previous generation in Update.
func (gf *GlobalFilter) Close() {
	gf.pipelines.Range(func(key, value interface{}) bool {
		if v, ok := value.(*httppipeline.HTTPPipeline); ok {
			v.Close()
		}
		return true
	})
	gf.pipelines.Delete(beforePipelineKey)
	gf.pipelines.Delete(afterPipelineKey)
}

// Validate validates Spec.
func (s *Spec) Validate() (err error) {

	err = s.BeforePipeline.Validate()
	if err != nil {
		return fmt.Errorf("before pipeline is invalidate err: %v", err)
	}
	err = s.AfterPipeline.Validate()
	if err != nil {
		return fmt.Errorf("after pipeline is invalidate err: %v", err)
	}

	return nil
}

func (gf *GlobalFilter) reload(previousGeneration *GlobalFilter) {
	var beforePreviousPipeline, afterPreviousPipeline *httppipeline.HTTPPipeline
	if previousGeneration != nil {
		gf.pipelines = previousGeneration.pipelines
	}
	// create and update beforePipeline entity
	if len(gf.spec.BeforePipeline.Flow) != 0 {
		if previousGeneration != nil {
			previous, ok := gf.pipelines.Load(beforePipelineKey)
			if ok {
				beforePreviousPipeline = previous.(*httppipeline.HTTPPipeline)
			}
		}
		err := gf.CreateAndUpdateBeforePipelineForSpec(gf.spec, beforePreviousPipeline)
		if err != nil {
			panic(fmt.Sprintf("create before pipeline error %v", err))
		}
	}
	//create and update afterPipeline entity
	if len(gf.spec.AfterPipeline.Flow) != 0 {
		if previousGeneration != nil {
			previous, ok := gf.pipelines.Load(beforePipelineKey)
			if ok {
				afterPreviousPipeline = previous.(*httppipeline.HTTPPipeline)
			}
		}
		err := gf.CreateAndUpdateAfterPipelineForSpec(gf.spec, afterPreviousPipeline)
		if err != nil {
			panic(fmt.Sprintf("create after pipeline error %v", err))
		}
	}
}
