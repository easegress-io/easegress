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
	"reflect"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of Pipeline.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of Pipeline.
	Kind = "Pipeline"
)

func init() {
	supervisor.Register(&Pipeline{})
}

type (
	Pipeline struct {
		superSpec      *supervisor.Spec
		spec           *Spec
		runningFilters []*runningFilter
	}

	runningFilter struct {
		spec       *FilterSpec
		rootFilter Filter
		filter     Filter
	}
)

var _ supervisor.Controller = (*Pipeline)(nil)

func (p *Pipeline) Category() supervisor.ObjectCategory {
	return Category
}

func (p *Pipeline) Kind() string {
	return Kind
}

func (p *Pipeline) DefaultSpec() interface{} {
	return nil
}

func (p *Pipeline) Status() *supervisor.Status {
	return &supervisor.Status{}
}

func (p *Pipeline) Close() {
	deletePipeline(p.spec.Name, p.spec.Protocol)
}

func (p *Pipeline) Init(superSpec *supervisor.Spec) {
	p.superSpec, p.spec, p.spec.Name = superSpec, superSpec.ObjectSpec().(*Spec), superSpec.Name()
	p.reload(nil /*no previous generation*/)
	storePipeline(p.spec.Name, p.spec.Protocol, p)
}

func (p *Pipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	p.superSpec, p.spec, p.spec.Name = superSpec, superSpec.ObjectSpec().(*Spec), superSpec.Name()
	prev := previousGeneration.(*Pipeline)
	deletePipeline(prev.spec.Name, p.spec.Protocol)
	p.reload(prev)
	storePipeline(p.spec.Name, p.spec.Protocol, p)
}

func (p *Pipeline) HandleMQTT(ctx context.MQTTContext) {
	if p.spec.Protocol != context.MQTT {
		logger.Errorf("pipeline %s not support protocol MQTT but %s", p.spec.Name, p.spec.Protocol)
	}
	for _, rf := range p.runningFilters {
		f := rf.filter.(MQTTFilter)
		f.HandleMQTT(ctx)
	}
}

func (p *Pipeline) reload(previousGeneration *Pipeline) {
	runningFilters := make([]*runningFilter, 0)
	if len(p.spec.Flow) == 0 {
		for _, filterSpec := range p.spec.Filters {
			spec, err := NewFilterSpec(filterSpec, p.superSpec.Super())
			if err != nil {
				panic(err)
			}

			runningFilters = append(runningFilters, &runningFilter{
				spec: spec,
			})
		}
	} else {
		filterMap := make(map[string]*FilterSpec)
		for _, filterSpec := range p.spec.Filters {
			spec, err := NewFilterSpec(filterSpec, p.superSpec.Super())
			if err != nil {
				panic(err)
			}
			filterMap[spec.Name()] = spec
		}
		for _, f := range p.spec.Flow {
			if spec, ok := filterMap[f.Filter]; ok {
				runningFilters = append(runningFilters, &runningFilter{
					spec: spec,
				})
			} else {
				panic(fmt.Errorf("flow filter %s not found in filters", f.Filter))
			}
		}
	}

	for _, runningFilter := range runningFilters {
		name, kind := runningFilter.spec.Name(), runningFilter.spec.Kind()
		rootFilter, exists := filterRegistry[kind]
		if !exists {
			panic(fmt.Errorf("kind %s not found", kind))
		}

		var prevInstance Filter
		if previousGeneration != nil {
			runningFilter := previousGeneration.getRunningFilter(name)
			if runningFilter != nil {
				prevInstance = runningFilter.filter
			}
		}

		filter := reflect.New(reflect.TypeOf(rootFilter).Elem()).Interface().(Filter)
		runningFilter.spec.meta.Pipeline = p.spec.Name
		if prevInstance == nil {
			filter.Init(runningFilter.spec)
		} else {
			filter.Inherit(runningFilter.spec, prevInstance)
		}
		runningFilter.filter, runningFilter.rootFilter = filter, rootFilter

	}
	p.runningFilters = runningFilters
	p.checkProtocol()
}

func (p *Pipeline) checkProtocol() {
	for _, rf := range p.runningFilters {
		protocol, err := getProtocol(rf.filter)
		if err != nil {
			panic(err)
		}
		if protocol != p.spec.Protocol {
			panic(fmt.Errorf("filter %v not support pipeline protocol %s", rf.spec.Name(), p.spec.Protocol))
		}
	}
}

func (p *Pipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range p.runningFilters {
		if filter.spec.Name() == name {
			return filter
		}
	}
	return nil
}