package httppipeline

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/supervisor"
	"github.com/megaease/easegateway/pkg/util/stringtool"

	yaml "gopkg.in/yaml.v2"
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
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

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
		Filters []map[string]interface{} `yaml:"filters" jsonschema:"-"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Filter string            `yaml:"filter" jsonschema:"required,format=urlname"`
		JumpIf map[string]string `yaml:"jumpIf" jsonschema:"omitempty"`
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		Health string `yaml:"health"`

		Filters map[string]interface{} `yaml:"filters"`
	}

	// PipelineContext contains the context of the HTTPPipeline.
	PipelineContext struct {
		FilterStats []*FilterStat
	}

	// FilterStat records the statistics of the running filter.
	FilterStat struct {
		Name     string
		Kind     string
		Result   string
		Duration time.Duration
	}
)

func (ps *FilterStat) log() string {
	result := ps.Result
	if result != "" {
		result += ","
	}
	return stringtool.Cat(ps.Name, "(", result, ps.Duration.String(), ")")
}

func (ctx *PipelineContext) log() string {
	if len(ctx.FilterStats) == 0 {
		return "<empty>"
	}

	logs := make([]string, len(ctx.FilterStats))
	for i, filterStat := range ctx.FilterStats {
		logs[i] = filterStat.log()
	}

	return strings.Join(logs, "->")
}

var (
	// context.HTTPContext: *PipelineContext
	runningContexts sync.Map = sync.Map{}
)

func newAndSetPipelineContext(ctx context.HTTPContext) *PipelineContext {
	pipeCtx := &PipelineContext{}

	runningContexts.Store(ctx, pipeCtx)

	return pipeCtx
}

// GetPipelineContext returns the corresponding PipelineContext of the HTTPContext,
// and a bool flag to represent it succeed or not.
func GetPipelineContext(ctx context.HTTPContext) (*PipelineContext, bool) {
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

func deletePipelineContext(ctx context.HTTPContext) {
	runningContexts.Delete(ctx)
}

func marshal(i interface{}) []byte {
	buff, err := yaml.Marshal(i)
	if err != nil {
		panic(fmt.Errorf("marsharl %#v failed: %v", i, err))
	}
	return buff
}

func unmarshal(buff []byte, i interface{}) {
	err := yaml.Unmarshal(buff, i)
	if err != nil {
		panic(fmt.Errorf("unmarshal failed: %v", err))
	}
}

func extractFiltersData(config []byte) interface{} {
	var whole map[string]interface{}
	unmarshal(config, &whole)
	return whole["filters"]
}

func convertToFilterBuffs(obj interface{}) map[string][]byte {
	var filters []map[string]interface{}
	unmarshal(marshal(obj), &filters)

	rst := make(map[string][]byte)
	for _, p := range filters {
		buff := marshal(p)
		meta := &FilterMetaSpec{}
		unmarshal(buff, meta)
		rst[meta.Name] = buff
	}
	return rst
}

func (meta *FilterMetaSpec) validate() error {
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
func (s Spec) Validate(config []byte) (err error) {
	errPrefix := "filters"
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %s", errPrefix, r)
		}
	}()

	filtersData := extractFiltersData(config)
	if filtersData == nil {
		return fmt.Errorf("validate failed: filters is required")
	}
	filterBuffs := convertToFilterBuffs(filtersData)

	filterSpecs := make(map[string]*FilterSpec)
	var templateFilterBuffs []context.FilterBuff
	for _, filterSpec := range s.Filters {
		spec, err := newFilterSpecInternal(filterSpec)
		if err != nil {
			panic(err)
		}

		if _, exists := filterSpecs[spec.Name()]; exists {
			panic(fmt.Errorf("conflict name: %s", spec.Name()))
		}
		filterSpecs[spec.Name()] = spec

		templateFilterBuffs = append(templateFilterBuffs, context.FilterBuff{
			Name: spec.Name(),
			Buff: filterBuffs[spec.Name()],
		})
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

// Init initilizes HTTPPipeline.
func (hp *HTTPPipeline) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	hp.superSpec, hp.spec, hp.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	hp.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of HTTPPipeline.
func (hp *HTTPPipeline) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	hp.superSpec, hp.spec, hp.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	hp.reload(previousGeneration.(*HTTPPipeline))

	// NOTE: It's filters' responsibility to inherit and clean their resources.
	// previousGeneration.Close()
}

func (hp *HTTPPipeline) reload(previousGeneration *HTTPPipeline) {
	runningFilters := make([]*runningFilter, 0)
	if len(hp.spec.Flow) == 0 {
		for _, filterSpec := range hp.spec.Filters {
			spec, err := newFilterSpecInternal(filterSpec)
			if err != nil {
				panic(err)
			}

			runningFilters = append(runningFilters, &runningFilter{
				spec: spec,
			})
		}
	} else {
		for _, f := range hp.spec.Flow {
			var spec *FilterSpec
			for _, filterSpec := range hp.spec.Filters {
				var err error
				spec, err = newFilterSpecInternal(filterSpec)
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
		if prevInstance == nil {
			filter.Init(runningFilter.spec, hp.super)
		} else {
			filter.Inherit(runningFilter.spec, prevInstance, hp.super)
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

// Handle handles all incoming traffic.
func (hp *HTTPPipeline) Handle(ctx context.HTTPContext) {
	pipeCtx := newAndSetPipelineContext(ctx)
	defer deletePipelineContext(ctx)

	// Here is the truly initialed HTTPTemplate by HTTPPipeline's filter
	// specs
	ctx.SetTemplate(hp.ht)
	nextFilterName := hp.runningFilters[0].spec.Name()
	for i := 0; i < len(hp.runningFilters); i++ {
		if nextFilterName == LabelEND {
			break
		}

		runningFilter := hp.runningFilters[i]
		if nextFilterName != runningFilter.spec.Name() {
			continue
		}

		if err := ctx.SaveReqToTemplate(runningFilter.spec.Name()); err != nil {
			logger.Errorf("save http req failed, dict is %#v err is %v",
				ctx.Template().GetDict(), err)
		}

		startTime := time.Now()
		result := runningFilter.filter.Handle(ctx)
		handleDuration := time.Now().Sub(startTime)

		if err := ctx.SaveRspToTemplate(runningFilter.spec.Name()); err != nil {
			logger.Errorf("save http rsp failed, dict is %#v err is %v",
				ctx.Template().GetDict(), err)
		}

		filterStat := &FilterStat{
			Name:     runningFilter.spec.Name(),
			Kind:     runningFilter.spec.Kind(),
			Result:   result,
			Duration: handleDuration,
		}
		pipeCtx.FilterStats = append(pipeCtx.FilterStats, filterStat)

		if result != "" {
			if !stringtool.StrInSlice(result, runningFilter.rootFilter.Results()) {
				logger.Errorf("BUG: invalid result %s not in %v",
					result, runningFilter.rootFilter.Results())
			}

			jumpIf := runningFilter.jumpIf
			if len(jumpIf) == 0 {
				break
			}
			var exists bool
			nextFilterName, exists = jumpIf[result]
			if !exists {
				break
			}
		} else if i < len(hp.runningFilters)-1 {
			nextFilterName = hp.runningFilters[i+1].spec.Name()
		}
	}

	ctx.AddTag(stringtool.Cat("pipeline: ", pipeCtx.log()))
}

func (hp *HTTPPipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range hp.runningFilters {
		if filter.spec.Name() == name {
			return filter
		}
	}

	return nil
}

// Status returns Status genreated by Runtime.
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
