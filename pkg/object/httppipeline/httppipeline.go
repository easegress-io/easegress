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
	"github.com/megaease/easegateway/pkg/v"

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
		spec *Spec

		runningFilters []*runningFilter
		ht             *context.HTTPTemplate
	}

	runningFilter struct {
		spec   map[string]interface{}
		jumpIf map[string]string
		filter Filter
		meta   *FilterMeta
		fr     *FilterRecord
	}

	// Spec describes the HTTPPipeline.
	Spec struct {
		supervisor.ObjectMetaSpec `yaml:",inline"`

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
		meta := &FilterMeta{}
		unmarshal(buff, meta)
		rst[meta.Name] = buff
	}
	return rst
}

func validateFilterMeta(meta *FilterMeta) error {
	if len(meta.Name) == 0 {
		return fmt.Errorf("filter name is required")
	}
	if len(meta.Kind) == 0 {
		return fmt.Errorf("filter kind is required")
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

	filterRecords := make(map[string]*FilterRecord)
	var templateFilterBuffs []context.FilterBuff
	for _, filter := range s.Filters {
		buff := marshal(filter)

		meta := &FilterMeta{}
		unmarshal(buff, meta)
		err := validateFilterMeta(meta)
		if err != nil {
			panic(err)
		}
		if meta.Name == LabelEND {
			panic(fmt.Errorf("can't use %s(built-in label) for filter name", LabelEND))
		}

		if _, exists := filterRecords[meta.Name]; exists {
			panic(fmt.Errorf("conflict name: %s", meta.Name))
		}

		fr, exists := filterBook[meta.Kind]
		if !exists {
			panic(fmt.Errorf("filters: unsuppoted kind %s", meta.Kind))
		}
		filterRecords[meta.Name] = fr

		filterSpec := reflect.ValueOf(fr.DefaultSpecFunc).Call(nil)[0].Interface()
		unmarshal(buff, filterSpec)
		vr := v.Validate(filterSpec, filterBuffs[meta.Name])
		if !vr.Valid() {
			panic(vr)
		}
		err = nil
		if fr == nil {
			panic(fmt.Errorf("filter kind %s not found", filter["kind"]))
		}
		templateFilterBuffs = append(templateFilterBuffs, context.FilterBuff{Name: meta.Name, Buff: filterBuffs[meta.Name]})
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

	labelsValid := map[string]struct{}{LabelEND: struct{}{}}
	for i := len(s.Flow) - 1; i >= 0; i-- {
		f := s.Flow[i]
		fr, exists := filterRecords[f.Filter]
		if !exists {
			panic(fmt.Errorf("filter %s not found", f.Filter))
		}
		for result, label := range f.JumpIf {
			if !stringtool.StrInSlice(result, fr.Results) {
				panic(fmt.Errorf("filter %s: result %s is not in %v",
					f.Filter, result, fr.Results))
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
func (hp *HTTPPipeline) DefaultSpec() supervisor.ObjectSpec {
	return &Spec{}
}

// Renew renews HTTPPipeline.
func (hp *HTTPPipeline) Renew(spec supervisor.ObjectSpec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	hp.spec = spec.(*Spec)

	runningFilters := make([]*runningFilter, 0)
	if len(hp.spec.Flow) == 0 {
		for _, filterSpec := range hp.spec.Filters {
			runningFilters = append(runningFilters, &runningFilter{
				spec: filterSpec,
			})
		}
	} else {
		for _, f := range hp.spec.Flow {
			var filterSpec map[string]interface{}
			for _, ps := range hp.spec.Filters {
				buff := marshal(ps)
				meta := &FilterMeta{}
				unmarshal(buff, meta)
				if meta.Name == f.Filter {
					filterSpec = ps
					break
				}
			}
			if filterSpec == nil {
				panic(fmt.Errorf("flow filter %s not found in filters", f.Filter))
			}
			runningFilters = append(runningFilters, &runningFilter{
				spec:   filterSpec,
				jumpIf: f.JumpIf,
			})
		}
	}

	var filterBuffs []context.FilterBuff
	for _, runningFilter := range runningFilters {
		buff := marshal(runningFilter.spec)

		meta := &FilterMeta{}
		unmarshal(buff, meta)

		fr, exists := filterBook[meta.Kind]
		if !exists {
			panic(fmt.Errorf("kind %s not found", meta.Kind))
		}

		defaultFilterSpec := reflect.ValueOf(fr.DefaultSpecFunc).Call(nil)[0].Interface()
		unmarshal(buff, defaultFilterSpec)

		prevValue := reflect.New(fr.FilterType).Elem()
		if previousGeneration != nil {
			prevFilter := previousGeneration.(*HTTPPipeline).getRunningFilter(meta.Name)
			if prevFilter != nil {
				prevValue = reflect.ValueOf(prevFilter.filter)
			}
		}
		in := []reflect.Value{reflect.ValueOf(defaultFilterSpec), prevValue}

		filter := reflect.ValueOf(fr.NewFunc).Call(in)[0].Interface().(Filter)

		runningFilter.filter, runningFilter.meta, runningFilter.fr = filter, meta, fr

		filterBuffs = append(filterBuffs, context.FilterBuff{Name: meta.Name, Buff: buff})
	}

	// creating a valid httptemplates
	var err error
	hp.ht, err = context.NewHTTPTemplate(filterBuffs)
	if err != nil {
		panic(fmt.Errorf("create http template failed %v", err))
	}

	hp.runningFilters = runningFilters

	if previousGeneration != nil {
		for _, runningFilter := range previousGeneration.(*HTTPPipeline).runningFilters {
			if hp.getRunningFilter(runningFilter.meta.Name) == nil {
				runningFilter.filter.Close()
			}
		}
	}
}

// Handle handles all incoming traffic.
func (hp *HTTPPipeline) Handle(ctx context.HTTPContext) {
	pipeCtx := newAndSetPipelineContext(ctx)
	defer deletePipelineContext(ctx)

	// Here is the truly initialed HTTPTemplate by HTTPPipeline's filter
	// specs
	ctx.SetTemplate(hp.ht)
	nextFilterName := hp.runningFilters[0].meta.Name
	for i := 0; i < len(hp.runningFilters); i++ {
		if nextFilterName == LabelEND {
			break
		}

		runningFilter := hp.runningFilters[i]
		if nextFilterName != runningFilter.meta.Name {
			continue
		}

		if err := ctx.SaveReqToTemplate(runningFilter.meta.Name); err != nil {
			logger.Errorf("save http req failed, dict is %#v err is %v",
				ctx.Template().GetDict(), err)
		}

		startTime := time.Now()
		result := runningFilter.filter.Handle(ctx)
		handleDuration := time.Now().Sub(startTime)

		if err := ctx.SaveRspToTemplate(runningFilter.meta.Name); err != nil {
			logger.Errorf("save http rsp failed, dict is %#v err is %v",
				ctx.Template().GetDict(), err)
		}

		filterStat := &FilterStat{
			Name:     runningFilter.meta.Name,
			Kind:     runningFilter.meta.Kind,
			Result:   result,
			Duration: handleDuration,
		}
		pipeCtx.FilterStats = append(pipeCtx.FilterStats, filterStat)

		if result != "" {
			if !stringtool.StrInSlice(result, runningFilter.fr.Results) {
				logger.Errorf("BUG: invalid result %s not in %v",
					result, runningFilter.fr.Results)
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
			nextFilterName = hp.runningFilters[i+1].meta.Name
		}
	}

	ctx.AddTag(stringtool.Cat("pipeline: ", pipeCtx.log()))
}

func (hp *HTTPPipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range hp.runningFilters {
		if filter.meta.Name == name {
			return filter
		}
	}

	return nil
}

// Status returns Status genreated by Runtime.
func (hp *HTTPPipeline) Status() interface{} {
	s := &Status{
		Filters: make(map[string]interface{}),
	}

	for _, runningFilter := range hp.runningFilters {
		filterStatus := reflect.ValueOf(runningFilter.filter).
			MethodByName("Status").Call(nil)[0].Interface()
		s.Filters[runningFilter.meta.Name] = filterStatus
	}

	return s
}

// Close closes HTTPPipeline.
func (hp *HTTPPipeline) Close() {
	for _, runningFilter := range hp.runningFilters {
		runningFilter.filter.Close()
	}
}
