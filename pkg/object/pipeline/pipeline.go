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
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Category is the category of Pipeline.
	Category = supervisor.CategoryPipeline

	// Kind is the kind of Pipeline.
	Kind = "Pipeline"

	// LabelEND is the built-in label for jumping of flow.
	LabelEND = "END"
)

func init() {
	supervisor.Register(&Pipeline{})
}

type (
	// Pipeline is Object Pipeline.
	Pipeline struct {
		superSpec *supervisor.Spec
		spec      *Spec

		filters []*filterInstance
		ht      *context.HTTPTemplate
	}

	filterInstance struct {
		spec       *FilterSpec
		requestID  string
		responseID string
		useRequest string
		jumpIf     map[string]string
		instance   Filter
	}

	// Spec describes the Pipeline.
	Spec struct {
		Flow    []Flow                   `yaml:"flow" jsonschema:"omitempty"`
		Filters []map[string]interface{} `yaml:"filters" jsonschema:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Filter     string            `yaml:"filter" jsonschema:"required,format=urlname"`
		RequestID  string            `yaml:"requestID" jsonschema:"requestID,omitempty"`
		ResponseID string            `yaml:"responseID" jsonschema:"responseID,omitempty"`
		UseRequest string            `yaml:"useRequest" jsonschema:"useRequest,omitempty"`
		JumpIf     map[string]string `yaml:"jumpIf" jsonschema:"omitempty"`
	}

	// Status is the status of Pipeline.
	Status struct {
		Health string `yaml:"health"`

		Filters map[string]interface{} `yaml:"filters"`
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

var filterStatPool = sync.Pool{
	New: func() interface{} {
		return &FilterStat{}
	},
}

func newFilterStat() *FilterStat {
	return filterStatPool.Get().(*FilterStat)
}

func releaseFilterStat(fs *FilterStat) {
	fs.Next = nil
	filterStatPool.Put(fs)
}

func (fs *FilterStat) marshalAndRelease() string {
	defer releaseFilterStat(fs)
	if len(fs.Next) == 0 {
		return "pipeline: <empty>"
	}

	var buf strings.Builder
	buf.WriteString("pipeline: ")

	var fn func(stat *FilterStat)
	fn = func(stat *FilterStat) {
		defer releaseFilterStat(stat)

		buf.WriteString(stat.Name)
		buf.WriteByte('(')
		if stat.Result != "" {
			buf.WriteString(stat.Result)
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

	fn(fs.Next[0])
	return buf.String()
}

// FlowFilterNames returns the filter names of the flow
func (s Spec) FlowFilterNames() []string {
	names := make([]string, len(s.Flow))
	for i, f := range s.Flow {
		names[i] = f.Filter
	}
	return names
}

// Validate validates Spec.
func (s Spec) Validate() (err error) {
	errPrefix := "filters"
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %s", errPrefix, r)
		}
	}()

	filterSpecs := map[string]*FilterSpec{}
	filterNames := []string{}

	// validate filter spec
	for _, f := range s.Filters {
		// NOTE: Nil supervisor is fine in spec validating phrase.
		spec, err := NewFilterSpec(f, nil)
		if err != nil {
			panic(err)
		}

		if spec.meta.Name == LabelEND {
			return fmt.Errorf("can't use %s(built-in label) for filter name", LabelEND)
		}

		filterSpecs[spec.meta.Name] = spec
		filterNames = append(filterNames, spec.meta.Name)
	}

	// validate flow
	errPrefix = "flow"
	filterOrder := []string{}
	validLabels := map[string]struct{}{LabelEND: {}}
	for i := len(s.Flow) - 1; i >= 0; i-- {
		f := s.Flow[i]
		if strings.EqualFold(f.Filter, "end") {
			continue
		}
		spec, exists := filterSpecs[f.Filter]
		if !exists {
			panic(fmt.Errorf("filter %s not found", f.Filter))
		}
		results := QueryFilterRegistry(spec.Kind()).Results()
		for result, label := range f.JumpIf {
			if !stringtool.StrInSlice(result, results) {
				panic(fmt.Errorf("filter %s: result %s is not in %v", f.Filter, result, results))
			}
			if _, exists := validLabels[label]; !exists {
				panic(fmt.Errorf("filter %s: label %s not found", f.Filter, label))
			}
		}
		validLabels[f.Filter] = struct{}{}
		filterOrder = append(filterOrder, f.Filter)
	}

	// TODO: remove below code?

	// sort filters using the Flow or the order they were defined
	if len(filterOrder) == 0 {
		filterOrder = filterNames
	}
	templateFilterBuffs := make([]context.FilterBuff, len(filterOrder))
	for i, name := range filterOrder {
		spec := filterSpecs[name]
		templateFilterBuffs[i] = context.FilterBuff{
			Name: name,
			Buff: []byte(spec.yamlConfig),
		}
	}
	// validate http template inside filter specs
	_, err = context.NewHTTPTemplate(templateFilterBuffs)
	if err != nil {
		panic(fmt.Errorf("filter has invalid httptemplate: %v", err))
	}

	return nil
}

// Category returns the category of Pipeline.
func (hp *Pipeline) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Pipeline.
func (hp *Pipeline) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Pipeline.
func (hp *Pipeline) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes Pipeline.
func (hp *Pipeline) Init(superSpec *supervisor.Spec, muxMapper protocols.MuxMapper) {
	hp.superSpec, hp.spec = superSpec, superSpec.ObjectSpec().(*Spec)

	hp.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of Pipeline.
func (hp *Pipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocols.MuxMapper) {
	hp.superSpec, hp.spec = superSpec, superSpec.ObjectSpec().(*Spec)

	hp.reload(previousGeneration.(*Pipeline))

	// NOTE: It's filters' responsibility to inherit and clean their resources.
	// previousGeneration.Close()
}

// creates FilterSpecs from a list of filters
func filtersToFilterSpecs(filters []map[string]interface{}, super *supervisor.Supervisor) (map[string]*FilterSpec, []string) {
	filterMap := make(map[string]*FilterSpec)
	filterNames := make([]string, 0)
	for _, filter := range filters {
		spec, err := NewFilterSpec(filter, super)
		if err != nil {
			panic(err)
		}
		if _, exists := filterMap[spec.Name()]; exists {
			panic(fmt.Errorf("conflict name: %s", spec.Name()))
		}
		filterMap[spec.Name()] = spec
		filterNames = append(filterNames, spec.Name())
	}
	return filterMap, filterNames
}

// Transforms map of FilterSpecs to list,
// sorted by Flow if present and otherwise in the order filters were defined
func filterSpecMapToSortedList(filterMap map[string]*FilterSpec, flowItems []Flow, filterNames []string) []*filterInstance {
	filters := make([]*filterInstance, len(filterNames))
	if len(flowItems) == 0 {
		for i, filterName := range filterNames {
			spec, _ := filterMap[filterName]
			filters[i] = &filterInstance{
				spec: spec,
			}
		}
	} else {
		for i, f := range flowItems {
			spec, specDefined := filterMap[f.Filter]
			if !specDefined {
				panic(fmt.Errorf("flow filter %s not found in filters", f.Filter))
			}

			filters[i] = &filterInstance{
				spec:   spec,
				jumpIf: f.JumpIf,
			}
		}
	}
	return filters
}

func (hp *Pipeline) reload(previousGeneration *Pipeline) {
	filterSpecMap, filterNames := filtersToFilterSpecs(hp.spec.Filters, hp.superSpec.Super())
	filters := filterSpecMapToSortedList(filterSpecMap, hp.spec.Flow, filterNames)

	pipelineName := hp.superSpec.Name()
	var filterBuffs []context.FilterBuff
	for _, filter := range filters {
		name, kind := filter.spec.Name(), filter.spec.Kind()
		rootFilter := QueryFilterRegistry(kind)
		if rootFilter == nil {
			panic(fmt.Errorf("kind %s not found", kind))
		}

		var prevInstance Filter
		if previousGeneration != nil {
			prevFilter := previousGeneration.getFilter(name)
			if prevFilter != nil {
				prevInstance = prevFilter.instance
			}
		}

		instance := reflect.New(reflect.TypeOf(rootFilter).Elem()).Interface().(Filter)
		filter.spec.pipeline = pipelineName
		if prevInstance == nil {
			instance.Init(filter.spec)
		} else {
			instance.Inherit(filter.spec, prevInstance)
		}

		filter.instance = instance

		filterBuffs = append(filterBuffs, context.FilterBuff{
			Name: name,
			Buff: []byte(filter.spec.YAMLConfig()),
		})
	}

	// creating a valid httptemplates
	var err error
	hp.ht, err = context.NewHTTPTemplate(filterBuffs)
	if err != nil {
		panic(fmt.Errorf("create http template failed %v", err))
	}

	hp.filters = filters
}

// getNextFilterIndex return filter index and whether jumped to the end of the pipeline.
func (hp *Pipeline) getNextFilterIndex(index int, result string) (int, bool) {
	// return index + 1 if last filter succeeded
	if result == "" {
		return index + 1, false
	}

	// check the jumpIf table of current filter, return its index if the jump
	// target is valid and -1 otherwise
	filter := hp.filters[index]
	if !stringtool.StrInSlice(result, filter.instance.Results()) {
		format := "BUG: invalid result %s not in %v"
		logger.Errorf(format, result, filter.instance.Results())
	}

	if len(filter.jumpIf) == 0 {
		return -1, false
	}
	name, ok := filter.jumpIf[result]
	if !ok {
		return -1, false
	}
	if name == LabelEND {
		return len(hp.filters), true
	}

	for index++; index < len(hp.filters); index++ {
		if hp.filters[index].spec.Name() == name {
			return index, false
		}
	}

	return -1, false
}

// Handle is the handler to deal with HTTP
func (hp *Pipeline) Handle(ctx context.HTTPContext) string {
	ctx.SetTemplate(hp.ht)

	filterIndex := -1
	filterStat := newFilterStat()
	isEnd := false

	handle := func(lastResult string) string {
		// For saving the `filterIndex`'s filter generated HTTP Response.
		// Note: the sequence of pipeline is stack-liked, we save the filter's response into template
		// at the beginning of the next filter.
		if filterIndex != -1 {
			name := hp.filters[filterIndex].spec.Name()
			if err := ctx.SaveRspToTemplate(name); err != nil {
				format := "save http rsp failed, dict is %#v err is %v"
				logger.Errorf(format, ctx.Template().GetDict(), err)
			}
			logger.LazyDebug(func() string {
				return fmt.Sprintf("filter %s, saved response dict %v", name, ctx.Template().GetDict())
			})
		}

		// Filters are called recursively as a stack, so we need to save current
		// state and restore it before return
		lastIndex := filterIndex
		lastStat := filterStat
		defer func() {
			filterIndex = lastIndex
			filterStat = lastStat
		}()

		filterIndex, isEnd = hp.getNextFilterIndex(filterIndex, lastResult)
		if isEnd {
			return LabelEND // jumpIf end of pipeline
		}
		if filterIndex == len(hp.filters) {
			return "" // reach the end of pipeline
		} else if filterIndex == -1 {
			return lastResult // an error occurs but no filter can handle it
		}

		filter := hp.filters[filterIndex]
		name := filter.spec.Name()

		if err := ctx.SaveReqToTemplate(name); err != nil {
			format := "save http req failed, dict is %#v err is %v"
			logger.Errorf(format, ctx.Template().GetDict(), err)
		}

		logger.LazyDebug(func() string {
			return fmt.Sprintf("filter %s saved request dict %v", name, ctx.Template().GetDict())
		})
		filterStat = newFilterStat()
		filterStat.Name = name
		filterStat.Kind = filter.spec.Kind()

		startTime := fasttime.Now()
		result := filter.instance.Handle(ctx)

		filterStat.Duration = fasttime.Since(startTime)
		filterStat.Result = result

		lastStat.Next = append(lastStat.Next, filterStat)
		return result
	}

	ctx.SetHandlerCaller(handle)
	result := handle("")

	ctx.AddTag(filterStat.marshalAndRelease())
	return result
}

func (hp *Pipeline) getFilter(name string) *filterInstance {
	for _, filter := range hp.filters {
		if filter.spec.Name() == name {
			return filter
		}
	}

	return nil
}

// Status returns Status generated by Runtime.
func (hp *Pipeline) Status() *supervisor.Status {
	s := &Status{
		Filters: make(map[string]interface{}),
	}

	for _, filter := range hp.filters {
		s.Filters[filter.spec.Name()] = filter.instance.Status()
	}

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes Pipeline.
func (hp *Pipeline) Close() {
	for _, filter := range hp.filters {
		filter.instance.Close()
	}
}
