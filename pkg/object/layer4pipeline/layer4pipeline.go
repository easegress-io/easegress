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

// context.TCPContext: *PipelineContext
var runningContexts = sync.Map{}

func newAndSetPipelineContext(ctx context.Layer4Context) *PipelineContext {
	pipeCtx := &PipelineContext{}
	runningContexts.Store(ctx, pipeCtx)
	return pipeCtx
}

// GetPipelineContext returns the corresponding PipelineContext of the TCPContext,
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
func (hp *Layer4Pipeline) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Layer4Pipeline.
func (hp *Layer4Pipeline) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Layer4Pipeline.
func (hp *Layer4Pipeline) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes Layer4Pipeline.
func (hp *Layer4Pipeline) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	hp.superSpec, hp.spec, hp.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	hp.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of Layer4Pipeline.
func (hp *Layer4Pipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	hp.superSpec, hp.spec, hp.muxMapper = superSpec, superSpec.ObjectSpec().(*Spec), muxMapper

	hp.reload(previousGeneration.(*Layer4Pipeline))

	// NOTE: It's filters' responsibility to inherit and clean their resources.
	// previousGeneration.Close()
}

func (hp *Layer4Pipeline) reload(previousGeneration *Layer4Pipeline) {
	runningFilters := make([]*runningFilter, 0)
	if len(hp.spec.Flow) == 0 {
		for _, filterSpec := range hp.spec.Filters {
			spec, err := NewFilterSpec(filterSpec, hp.superSpec.Super())
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
				spec, err = NewFilterSpec(filterSpec, hp.superSpec.Super())
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

	hp.runningFilters = runningFilters
}

func (hp *Layer4Pipeline) getNextFilterIndex(index int, result string) int {
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

// Handle is the handler to deal with layer4
func (hp *Layer4Pipeline) Handle(ctx context.Layer4Context) {
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

		filterIndex = hp.getNextFilterIndex(filterIndex, lastResult)
		if filterIndex == len(hp.runningFilters) {
			return "" // reach the end of pipeline
		} else if filterIndex == -1 {
			return lastResult // an error occurs but no filter can handle it
		}

		filter := hp.runningFilters[filterIndex]
		name := filter.spec.Name()
		filterStat = &FilterStat{Name: name, Kind: filter.spec.Kind()}

		startTime := time.Now()
		result := filter.filter.Handle(ctx)
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

func (hp *Layer4Pipeline) getRunningFilter(name string) *runningFilter {
	for _, filter := range hp.runningFilters {
		if filter.spec.Name() == name {
			return filter
		}
	}

	return nil
}

// Status returns Status generated by Runtime.
func (hp *Layer4Pipeline) Status() *supervisor.Status {
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

// Close closes Layer4Pipeline.
func (hp *Layer4Pipeline) Close() {
	for _, runningFilter := range hp.runningFilters {
		runningFilter.filter.Close()
	}
}
