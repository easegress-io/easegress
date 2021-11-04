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
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

const (
	// Category is the category of HTTPPipeline.
	Category = supervisor.CategoryPipeline

	// Kind is the kind of HTTPPipeline.
	Kind = "HTTPPipeline"

	// LabelEND is the built-in label for jumping of flow.
	LabelEND = "END"
)

func init() {
	supervisor.Register(&HTTPPipeline{})
}

type (
	// HTTPPipeline is Object HTTPPipeline.
	HTTPPipeline struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper      protocol.MuxMapper
		runningFilters []*runningFilter
		ht             *context.HTTPTemplate
	}

	runningFilter struct {
		spec       *FilterSpec
		jumpIf     map[string]string
		rootFilter Filter
		filter     Filter
	}

	// Spec describes the HTTPPipeline.
	Spec struct {
		Flow    []Flow                   `yaml:"flow" jsonschema:"omitempty"`
		Filters []map[string]interface{} `yaml:"filters" jsonschema:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Filter string            `yaml:"filter" jsonschema:"required,format=urlname"`
		JumpIf map[string]string `yaml:"jumpIf" jsonschema:"omitempty"`
	}

	// Status is the status of HTTPPipeline.
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

func extractFiltersData(config []byte) interface{} {
	var whole map[string]interface{}
	yamltool.Unmarshal(config, &whole)
	return whole["filters"]
}

func convertToFilterBuffs(obj interface{}) map[string][]byte {
	var filters []map[string]interface{}
	yamltool.Unmarshal(yamltool.Marshal(obj), &filters)

	rst := make(map[string][]byte)
	for _, p := range filters {
		buff := yamltool.Marshal(p)
		meta := &FilterMetaSpec{}
		yamltool.Unmarshal(buff, meta)
		rst[meta.Name] = buff
	}
	return rst
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

	config := yamltool.Marshal(s)

	filtersData := extractFiltersData(config)
	if filtersData == nil {
		return fmt.Errorf("filters is required")
	}
	filterBuffs := convertToFilterBuffs(filtersData)

	filterSpecs, filterNames := filtersToFilterSpecs(
		s.Filters,
		nil, /*NOTE: Nil supervisor is fine in spec validating phrase.*/
	)
	// sort filters using the Flow or the order they were defined
	filterOrder := s.FlowFilterNames()
	if len(filterOrder) == 0 {
		filterOrder = filterNames
	}
	templateFilterBuffs := make([]context.FilterBuff, len(filterOrder))
	for i, name := range filterOrder {
		templateFilterBuffs[i] = context.FilterBuff{
			Name: name, Buff: filterBuffs[name],
		}
	}
	// validate http template inside filter specs
	_, err = context.NewHTTPTemplate(templateFilterBuffs)
	if err != nil {
		panic(fmt.Errorf("filter has invalid httptemplate: %v", err))
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

// Category returns the category of HTTPPipeline.
func (hp *HTTPPipeline) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of HTTPPipeline.
func (hp *HTTPPipeline) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HTTPPipeline.
func (hp *HTTPPipeline) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes HTTPPipeline.
func (hp *HTTPPipeline) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	hp.superSpec, hp.spec, hp.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	hp.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of HTTPPipeline.
func (hp *HTTPPipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	hp.superSpec, hp.spec, hp.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	hp.reload(previousGeneration.(*HTTPPipeline))

	// NOTE: It's filters' responsibility to inherit and clean their resources.
	// previousGeneration.Close()
}

// creates FilterSpecs from a list of filters
func filtersToFilterSpecs(
	filters []map[string]interface{},
	super *supervisor.Supervisor,
) (map[string]*FilterSpec, []string) {
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
func filterSpecMapToSortedList(
	filterMap map[string]*FilterSpec, flowItems []Flow, filterNames []string) []*runningFilter {
	runningFilters := make([]*runningFilter, len(filterNames))
	if len(flowItems) == 0 {
		for i, filterName := range filterNames {
			spec, _ := filterMap[filterName]
			runningFilters[i] = &runningFilter{
				spec: spec,
			}
		}
	} else {
		for i, f := range flowItems {
			spec, specDefined := filterMap[f.Filter]
			if !specDefined {
				panic(fmt.Errorf("flow filter %s not found in filters", f.Filter))
			}

			runningFilters[i] = &runningFilter{
				spec:   spec,
				jumpIf: f.JumpIf,
			}
		}
	}
	return runningFilters
}

func (hp *HTTPPipeline) reload(previousGeneration *HTTPPipeline) {
	filterSpecMap, filterNames := filtersToFilterSpecs(hp.spec.Filters, hp.superSpec.Super())
	runningFilters := filterSpecMapToSortedList(filterSpecMap, hp.spec.Flow, filterNames)

	pipelineName := hp.superSpec.Name()
	var filterBuffs []context.FilterBuff
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

		filterBuffs = append(filterBuffs, context.FilterBuff{
			Name: name,
			Buff: []byte(runningFilter.spec.YAMLConfig()),
		})
	}

	// creating a valid httptemplates
	var err error
	hp.ht, err = context.NewHTTPTemplate(filterBuffs)
	if err != nil {
		panic(fmt.Errorf("create http template failed %v", err))
	}

	hp.runningFilters = runningFilters
}

func (hp *HTTPPipeline) getNextFilterIndex(index int, result string) int {
	// return index + 1 if last filter succeeded
	if result == "" {
		return index + 1
	}

	// check the jumpIf table of current filter, return its index if the jump
	// target is valid and -1 otherwise
	filter := hp.runningFilters[index]
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
		return len(hp.runningFilters)
	}

	for index++; index < len(hp.runningFilters); index++ {
		if hp.runningFilters[index].spec.Name() == name {
			return index
		}
	}

	return -1
}

// Handle is the handler to deal with HTTP
func (hp *HTTPPipeline) Handle(ctx context.HTTPContext) {
	ctx.SetTemplate(hp.ht)

	filterIndex := -1
	filterStat := newFilterStat()

	handle := func(lastResult string) string {
		// For saving the `filterIndex`'s filter generated HTTP Response.
		// Note: the sequence of pipeline is stack-liked, we save the filter's response into template
		// at the beginning of the next filter.
		if filterIndex != -1 {
			name := hp.runningFilters[filterIndex].spec.Name()
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

		filterIndex = hp.getNextFilterIndex(filterIndex, lastResult)
		if filterIndex == len(hp.runningFilters) {
			return "" // reach the end of pipeline
		} else if filterIndex == -1 {
			return lastResult // an error occurs but no filter can handle it
		}

		filter := hp.runningFilters[filterIndex]
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
		result := filter.filter.Handle(ctx)

		filterStat.Duration = fasttime.Since(startTime)
		filterStat.Result = result

		lastStat.Next = append(lastStat.Next, filterStat)
		return result
	}

	ctx.SetHandlerCaller(handle)
	handle("")

	ctx.AddTag(filterStat.marshalAndRelease())
}

func (hp *HTTPPipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range hp.runningFilters {
		if filter.spec.Name() == name {
			return filter
		}
	}

	return nil
}

// Status returns Status generated by Runtime.
func (hp *HTTPPipeline) Status() *supervisor.Status {
	s := &Status{
		Filters: make(map[string]interface{}),
	}

	for _, runningFilter := range hp.runningFilters {
		s.Filters[runningFilter.spec.Name()] = runningFilter.filter.Status()
	}

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes HTTPPipeline.
func (hp *HTTPPipeline) Close() {
	for _, runningFilter := range hp.runningFilters {
		runningFilter.filter.Close()
	}
}
