package httppipeline

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/registry"
	"github.com/megaease/easegateway/pkg/v"

	"gopkg.in/yaml.v2"
)

func init() {
	registry.Register(Kind, DefaultSpec)
}

const (
	// Kind is HTTPPipeline kind.
	Kind = "HTTPPipeline"

	// LabelEND is the built-in label for jumping of flow.
	LabelEND = "END"
)

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
	}

	// Closer is the common interface for plugins, which could not be implemented.
	Closer interface {
		Close()
	}

	// Spec describes the HTTPPipeline.
	Spec struct {
		V string `yaml:"-" v:"parent"`

		registry.MetaSpec `yaml:",inline"`

		Flow    []Flow                   `yaml:"flow"`
		Plugins []map[string]interface{} `yaml:"plugins" v:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Plugin string            `yaml:"plugin" v:"required"`
		JumpIf map[string]string `yaml:"jumpIf"`
	}
)

// DefaultSpec returns HTTPPipeline default spec.
func DefaultSpec() registry.Spec {
	return &Spec{}
}

func marshal(i interface{}) []byte {
	buff, err := yaml.Marshal(i)
	if err != nil {
		panic(err)
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
func New(spec *Spec, r *Runtime) *HTTPPipeline {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("BUG: %v", err)
		}
	}()

	hp := &HTTPPipeline{spec: spec}

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

		in := []reflect.Value{reflect.ValueOf(defaultPluginSpec)}
		if pr.needPrev {
			var prevValue reflect.Value
			prevPlugin := r.curr.getRunningPlugin(meta.Name)
			if prevPlugin == nil {
				prevValue = reflect.New(pr.pluginType).Elem()
			} else {
				prevValue = reflect.ValueOf(prevPlugin.plugin)
			}
			in = append(in, prevValue)
		}
		plugin := reflect.ValueOf(pr.NewFunc).Call(in)[0].Interface().(Plugin)

		runningPlugin.plugin, runningPlugin.meta, runningPlugin.pr = plugin, meta, pr
	}

	hp.runningPlugins = runningPlugins

	r.reload(hp)

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

// Close closes HTTPPipeline.
func (hp *HTTPPipeline) Close() {
}
