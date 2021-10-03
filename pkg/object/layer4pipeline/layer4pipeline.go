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

package layer4pipeline

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

const (
	// Category is the category of Layer4Pipeline.
	Category = supervisor.CategoryPipeline

	// Kind is the kind of Layer4Pipeline.
	Kind = "Layer4Pipeline"

	// LabelEND is the built-in label for jumping of flow.
	LabelEND = "END"
)

func init() {
	supervisor.Register(&Layer4Pipeline{})
}

type (
	// Layer4Pipeline is Object Layer4Pipeline.
	Layer4Pipeline struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper      protocol.MuxMapper
		runningFilters []*runningFilter
	}

	runningFilter struct {
		spec       *FilterSpec
		jumpIf     map[string]string
		rootFilter Filter
		filter     Filter
	}

	// Spec describes the Layer4Pipeline.
	Spec struct {
		Flow    []Flow                   `yaml:"flow" jsonschema:"omitempty"`
		Filters []map[string]interface{} `yaml:"filters" jsonschema:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Filter string            `yaml:"filter" jsonschema:"required,format=urlname"`
		JumpIf map[string]string `yaml:"jumpIf" jsonschema:"omitempty"`
	}

	// Status is the status of Layer4Pipeline.
	Status struct {
		Health string `yaml:"health"`

		Filters map[string]interface{} `yaml:"filters"`
	}

	// PipelineContext contains the context of the Layer4Pipeline.
	PipelineContext struct {
		FilterStats *FilterStat
	}

	// FilterStat records the statistics of the running filter.
	FilterStat struct {
		Name     string
		Kind     string
		Result   string
		Duration time.Duration
		Next     []*FilterStat
	}
)

func (fs *FilterStat) selfDuration() time.Duration {
	d := fs.Duration
	for _, s := range fs.Next {
		d -= s.Duration
	}
	return d
}

func (ctx *PipelineContext) log() string {
	if ctx.FilterStats == nil {
		return "<empty>"
	}

	var buf bytes.Buffer
	var fn func(stat *FilterStat)

	fn = func(stat *FilterStat) {
		buf.WriteString(stat.Name)
		buf.WriteByte('(')
		buf.WriteString(stat.Result)
		if stat.Result != "" {
			buf.WriteByte(',')
		}
		buf.WriteString(stat.selfDuration().String())
		buf.WriteByte(')')
		if len(stat.Next) == 0 {
			return
		}
		buf.WriteString("->")
		if len(stat.Next) > 1 {
			buf.WriteByte('[')
		}
		for i, s := range stat.Next {
			if i > 0 {
				buf.WriteByte(',')
			}
			fn(s)
		}
		if len(stat.Next) > 1 {
			buf.WriteByte(']')
		}
	}

	fn(ctx.FilterStats)
	return buf.String()
}

// context.Layer4Pipeline: *PipelineContext
var runningContexts = sync.Map{}

func newAndSetPipelineContext(ctx context.Layer4Context) *PipelineContext {
	pipeCtx := &PipelineContext{}
	runningContexts.Store(ctx, pipeCtx)
	return pipeCtx
}

// GetPipelineContext returns the corresponding PipelineContext of the Layer4Context,
// and a bool flag to represent it succeed or not.
func GetPipelineContext(ctx context.Layer4Context) (*PipelineContext, bool) {
	value, ok := runningContexts.Load(ctx)
	if !ok {
		return nil, false
	}

	pipeCtx, ok := value.(*PipelineContext)
	if !ok {
		logger.Errorf("BUG: want *PipelineContext, got %T", value)
		return nil, false
	}

	return pipeCtx, true
}

func deletePipelineContext(ctx context.Layer4Context) {
	runningContexts.Delete(ctx)
}

func extractFiltersData(config []byte) interface{} {
	var whole map[string]interface{}
	yamltool.Unmarshal(config, &whole)
	return whole["filters"]
}

// Validate validates the meta information
func (meta *FilterMetaSpec) Validate() error {
	if len(meta.Name) == 0 {
		return fmt.Errorf("filter name is required")
	}
	if len(meta.Kind) == 0 {
		return fmt.Errorf("filter kind is required")
	}

	if meta.Name == LabelEND {
		return fmt.Errorf("can't use %s(built-in label) for filter name", LabelEND)
	}
	return nil
}

// Validate validates Spec.
func (s Spec) Validate() (err error) {
	errPrefix := "filters"
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %s", errPrefix, r)
		}
	}()

	config := yamltool.Marshal(s)

	filtersData := extractFiltersData(config)
	if filtersData == nil {
		return fmt.Errorf("filters is required")
	}

	filterSpecs := make(map[string]*FilterSpec)
	for _, filterSpec := range s.Filters {
		// NOTE: Nil supervisor is fine in spec validating phrase.
		spec, err := NewFilterSpec(filterSpec, nil)
		if err != nil {
			panic(err)
		}

		if _, exists := filterSpecs[spec.Name()]; exists {
			panic(fmt.Errorf("conflict name: %s", spec.Name()))
		}
		filterSpecs[spec.Name()] = spec
	}

	errPrefix = "flow"
	filters := make(map[string]struct{})
	for _, f := range s.Flow {
		if _, exists := filters[f.Filter]; exists {
			panic(fmt.Errorf("repeated filter %s", f.Filter))
		}
	}

	labelsValid := map[string]struct{}{LabelEND: {}}
	for i := len(s.Flow) - 1; i >= 0; i-- {
		f := s.Flow[i]
		spec, exists := filterSpecs[f.Filter]
		if !exists {
			panic(fmt.Errorf("filter %s not found", f.Filter))
		}
		expectedResults := spec.RootFilter().Results()
		for result, label := range f.JumpIf {
			if !stringtool.StrInSlice(result, expectedResults) {
				panic(fmt.Errorf("filter %s: result %s is not in %v",
					f.Filter, result, expectedResults))
			}
			if _, exists := labelsValid[label]; !exists {
				panic(fmt.Errorf("filter %s: label %s not found",
					f.Filter, label))
			}
		}
		labelsValid[f.Filter] = struct{}{}
	}
	return nil
}

// Category returns the category of Layer4Pipeline.
func (l *Layer4Pipeline) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Layer4Pipeline.
func (l *Layer4Pipeline) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Layer4Pipeline.
func (l *Layer4Pipeline) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes Layer4Pipeline.
func (l *Layer4Pipeline) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	l.superSpec, l.spec, l.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	l.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of Layer4Pipeline.
func (l *Layer4Pipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	l.superSpec, l.spec, l.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	l.reload(previousGeneration.(*Layer4Pipeline))

	// NOTE: It's filters' responsibility to inherit and clean their resources.
	// previousGeneration.Close()
}

func (l *Layer4Pipeline) reload(previousGeneration *Layer4Pipeline) {
	runningFilters := make([]*runningFilter, 0)
	if len(l.spec.Flow) == 0 {
		for _, filterSpec := range l.spec.Filters {
			spec, err := NewFilterSpec(filterSpec, l.superSpec.Super())
			if err != nil {
				panic(err)
			}

			runningFilters = append(runningFilters, &runningFilter{
				spec: spec,
			})
		}
	} else {
		for _, f := range l.spec.Flow {
			var spec *FilterSpec
			for _, filterSpec := range l.spec.Filters {
				var err error
				spec, err = NewFilterSpec(filterSpec, l.superSpec.Super())
				if err != nil {
					panic(err)
				}
				if spec.Name() == f.Filter {
					break
				}
			}
			if spec == nil {
				panic(fmt.Errorf("flow filter %s not found in filters", f.Filter))
			}

			runningFilters = append(runningFilters, &runningFilter{
				spec:   spec,
				jumpIf: f.JumpIf,
			})
		}
	}

	pipelineName := l.superSpec.Name()
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
		runningFilter.spec.meta.Pipeline = pipelineName
		if prevInstance == nil {
			filter.Init(runningFilter.spec)
		} else {
			filter.Inherit(runningFilter.spec, prevInstance)
		}

		runningFilter.filter, runningFilter.rootFilter = filter, rootFilter
	}

	l.runningFilters = runningFilters
}

func (l *Layer4Pipeline) getNextFilterIndex(index int, result string) int {
	// return index + 1 if last filter succeeded
	if result == "" {
		return index + 1
	}

	// check the jumpIf table of current filter, return its index if the jump
	// target is valid and -1 otherwise
	filter := l.runningFilters[index]
	if !stringtool.StrInSlice(result, filter.rootFilter.Results()) {
		format := "BUG: invalid result %s not in %v"
		logger.Errorf(format, result, filter.rootFilter.Results())
	}

	if len(filter.jumpIf) == 0 {
		return -1
	}
	name, ok := filter.jumpIf[result]
	if !ok {
		return -1
	}
	if name == LabelEND {
		return len(l.runningFilters)
	}

	for index++; index < len(l.runningFilters); index++ {
		if l.runningFilters[index].spec.Name() == name {
			return index
		}
	}
	return -1
}

// InboundHandle is the handler to deal with layer4 inbound data
func (l *Layer4Pipeline) InboundHandle(ctx context.Layer4Context) {
	l.innerHandle(ctx, true)
}

// OutboundHandle is the handler to deal with layer4 outbound data
func (l *Layer4Pipeline) OutboundHandle(ctx context.Layer4Context) {
	l.innerHandle(ctx, false)
}

func (l *Layer4Pipeline) innerHandle(ctx context.Layer4Context, isInbound bool) {
	pipeCtx := newAndSetPipelineContext(ctx)
	defer deletePipelineContext(ctx)

	filterIndex := -1
	filterStat := &FilterStat{}

	handle := func(lastResult string) string {

		// Filters are called recursively as a stack, so we need to save current
		// state and restore it before return
		lastIndex := filterIndex
		lastStat := filterStat
		defer func() {
			filterIndex = lastIndex
			filterStat = lastStat
		}()

		filterIndex = l.getNextFilterIndex(filterIndex, lastResult)
		if filterIndex == len(l.runningFilters) {
			return "" // reach the end of pipeline
		} else if filterIndex == -1 {
			return lastResult // an error occurs but no filter can handle it
		}

		filter := l.runningFilters[filterIndex]
		name := filter.spec.Name()

		filterStat = &FilterStat{Name: name, Kind: filter.spec.Kind()}
		startTime := time.Now()
		var result string
		if isInbound {
			result = filter.filter.InboundHandle(ctx)
		} else {
			result = filter.filter.OutboundHandle(ctx)
		}
		filterStat.Duration = time.Since(startTime)
		filterStat.Result = result

		lastStat.Next = append(lastStat.Next, filterStat)
		return result
	}

	ctx.SetHandlerCaller(handle)
	handle("")

	if len(filterStat.Next) > 0 {
		pipeCtx.FilterStats = filterStat.Next[0]
	}
}

func (l *Layer4Pipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range l.runningFilters {
		if filter.spec.Name() == name {
			return filter
		}
	}
	return nil
}

// Status returns Status generated by Runtime.
func (l *Layer4Pipeline) Status() *supervisor.Status {
	s := &Status{
		Filters: make(map[string]interface{}),
	}

	for _, runningFilter := range l.runningFilters {
		s.Filters[runningFilter.spec.Name()] = runningFilter.filter.Status()
	}

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes Layer4Pipeline.
func (l *Layer4Pipeline) Close() {
	for _, runningFilter := range l.runningFilters {
		runningFilter.filter.Close()
	}
}
