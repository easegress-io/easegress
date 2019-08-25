package httppipeline

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/scheduler"
	"github.com/megaease/easegateway/pkg/v"

	yaml "gopkg.in/yaml.v2"
)

const (
	// Kind is HTTPPipeline kind.
	Kind = "HTTPPipeline"

	// LabelEND is the built-in label for jumping of flow.
	LabelEND = "END"
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: []string{httpserver.Kind},
	})
}

type (
	// HTTPPipeline is Object HTTPPipeline.
	HTTPPipeline struct {
		spec *Spec

		runningPlugins []*runningPlugin
	}

	runningPlugin struct {
		spec   map[string]interface{}
		jumpIf map[string]string
		plugin Plugin
		meta   *PluginMeta
		pr     *PluginRecord
	}

	// PluginMeta describes metadata of Plugin.
	PluginMeta struct {
		Name string `yaml:"name,omitempty"`
		Kind string `yaml:"kind,omitempty"`
	}

	// Plugin is the common interface for plugins handling HTTP traffic.
	Plugin interface {
		Handle(context.HTTPContext) (result string)
		Close()
	}

	// Spec describes the HTTPPipeline.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		scheduler.ObjectMeta `yaml:",inline"`

		Flow    []Flow                   `yaml:"flow"`
		Plugins []map[string]interface{} `yaml:"plugins" v:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Plugin string            `yaml:"plugin" v:"required"`
		JumpIf map[string]string `yaml:"jumpIf"`
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		Timestamp uint64 `yaml:"timestamp"`

		Plugins map[string]interface{} `yaml:"plugins"`
	}
)

// DefaultSpec returns HTTPPipeline default spec.
func DefaultSpec() *Spec {
	return &Spec{}
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

// Validate validates Spec.
func (s Spec) Validate() (err error) {
	errPrefix := "plugins"
	defer func() {
		if err1 := recover(); err1 != nil {
			err = fmt.Errorf("%s: %v", errPrefix, err1)
		}
	}()

	pluginRecords := make(map[string]*PluginRecord)
	for _, plugin := range s.Plugins {
		buff := marshal(plugin)

		meta := &PluginMeta{}
		unmarshal(buff, meta)
		err := v.Struct(meta)
		if err != nil {
			panic(err)
		}
		if meta.Name == LabelEND {
			panic(fmt.Errorf("can't use %s(built-in label) for plugin name", LabelEND))
		}

		if _, exists := pluginRecords[meta.Name]; exists {
			panic(fmt.Errorf("conflict name: %s", meta.Name))
		}

		pr, exists := pluginBook[meta.Kind]
		if !exists {
			panic(fmt.Errorf("plugins: unsuppoted kind %s", meta.Kind))
		}
		pluginRecords[meta.Name] = pr

		pluginSpec := reflect.ValueOf(pr.DefaultSpecFunc).Call(nil)[0].Interface()
		unmarshal(buff, pluginSpec)
		err = v.Struct(pluginSpec)
		if err != nil {
			panic(err)
		}

		if pr == nil {
			panic(fmt.Errorf("plugin kind %s not found", plugin["kind"]))
		}
	}

	errPrefix = "flow"

	plugins := make(map[string]struct{})
	for _, f := range s.Flow {
		if _, exists := plugins[f.Plugin]; exists {
			panic(fmt.Errorf("repeated plugin %s", f.Plugin))
		}
	}

	labelsValid := map[string]struct{}{LabelEND: struct{}{}}
	for i := len(s.Flow) - 1; i >= 0; i-- {
		f := s.Flow[i]
		pr, exists := pluginRecords[f.Plugin]
		if !exists {
			panic(fmt.Errorf("plugin %s not found", f.Plugin))
		}
		for result, label := range f.JumpIf {
			if !common.StrInSlice(result, pr.Results) {
				panic(fmt.Errorf("plugin %s: result %s is not in %v",
					f.Plugin, result, pr.Results))
			}
			if _, exists := labelsValid[label]; !exists {
				panic(fmt.Errorf("plugin %s: label %s not found",
					f.Plugin, label))
			}
		}
		labelsValid[f.Plugin] = struct{}{}
	}

	return nil
}

// New creates an HTTPPipeline
func New(spec *Spec, prev *HTTPPipeline, handlers *sync.Map) (tmp *HTTPPipeline) {
	hp := &HTTPPipeline{spec: spec}

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("BUG: %v", err)
		}
	}()

	runningPlugins := make([]*runningPlugin, 0)
	if len(spec.Flow) == 0 {
		for _, pluginSpec := range spec.Plugins {
			runningPlugins = append(runningPlugins, &runningPlugin{
				spec: pluginSpec,
			})
		}
	} else {
		for _, f := range spec.Flow {
			var pluginSpec map[string]interface{}
			for _, ps := range spec.Plugins {
				buff := marshal(ps)
				meta := &PluginMeta{}
				unmarshal(buff, meta)
				if meta.Name == f.Plugin {
					pluginSpec = ps
					break
				}
			}
			if pluginSpec == nil {
				panic(fmt.Errorf("flow plugin %s not found in plugins", f.Plugin))
			}
			runningPlugins = append(runningPlugins, &runningPlugin{
				spec:   pluginSpec,
				jumpIf: f.JumpIf,
			})
		}
	}

	for _, runningPlugin := range runningPlugins {
		buff := marshal(runningPlugin.spec)

		meta := &PluginMeta{}
		unmarshal(buff, meta)

		pr, exists := pluginBook[meta.Kind]
		if !exists {
			panic(fmt.Errorf("kind %s not found", meta.Kind))
		}

		defaultPluginSpec := reflect.ValueOf(pr.DefaultSpecFunc).Call(nil)[0].Interface()
		unmarshal(buff, defaultPluginSpec)

		prevValue := reflect.New(pr.pluginType).Elem()
		if prev != nil {
			prevPlugin := prev.getRunningPlugin(meta.Name)
			if prevPlugin != nil {
				prevValue = reflect.ValueOf(prevPlugin.plugin)
			}
		}
		in := []reflect.Value{reflect.ValueOf(defaultPluginSpec), prevValue}

		plugin := reflect.ValueOf(pr.NewFunc).Call(in)[0].Interface().(Plugin)

		runningPlugin.plugin, runningPlugin.meta, runningPlugin.pr = plugin, meta, pr
	}

	hp.runningPlugins = runningPlugins

	if prev != nil {
		for _, runningPlugin := range prev.runningPlugins {
			if hp.getRunningPlugin(runningPlugin.meta.Name) == nil {
				runningPlugin.plugin.Close()
			}
		}
	}

	handlers.Store(spec.Name, hp)

	return hp
}

// Handle handles all incoming traffic.
func (hp *HTTPPipeline) Handle(ctx context.HTTPContext) {
	pipeline := []string{}
	nextPluginName := hp.runningPlugins[0].meta.Name
	for i := 0; i < len(hp.runningPlugins); i++ {
		if nextPluginName == LabelEND {
			break
		}
		runningPlugin := hp.runningPlugins[i]
		if nextPluginName != runningPlugin.meta.Name {
			continue
		}
		startTime := time.Now()
		result := runningPlugin.plugin.Handle(ctx)
		pipeline = append(pipeline, fmt.Sprintf("%s(%v)",
			nextPluginName, time.Now().Sub(startTime)))
		if result != "" {
			jumpIf := runningPlugin.jumpIf
			if len(jumpIf) == 0 {
				break
			}
			var exists bool
			nextPluginName, exists = jumpIf[result]
			if !exists {
				break
			}
		} else if i < len(hp.runningPlugins)-1 {
			nextPluginName = hp.runningPlugins[i+1].meta.Name
		}
	}
	ctx.AddTag(fmt.Sprintf("pipeline: %s", strings.Join(pipeline, ",")))
}

func (hp *HTTPPipeline) getRunningPlugin(name string) *runningPlugin {
	for _, plugin := range hp.runningPlugins {
		if plugin.meta.Name == name {
			return plugin
		}
	}

	return nil
}

// Status returns Status genreated by Runtime.
// NOTE: Caller must not call Status while Newing.
func (hp *HTTPPipeline) Status() *Status {
	s := &Status{
		Plugins: make(map[string]interface{}),
	}

	for _, runningPlugin := range hp.runningPlugins {
		pluginStatus := reflect.ValueOf(runningPlugin.plugin).
			MethodByName("Status").Call(nil)[0].Interface()
		s.Plugins[runningPlugin.meta.Name] = pluginStatus
	}

	return s
}

// Close closes HTTPPipeline.
func (hp *HTTPPipeline) Close() {
	for _, runningPlugin := range hp.runningPlugins {
		runningPlugin.plugin.Close()
	}
}
