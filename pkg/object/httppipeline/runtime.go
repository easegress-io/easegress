package httppipeline

import (
	"reflect"

	"github.com/megaease/easegateway/pkg/logger"
)

type (

	// Runtime contains all runtime info of HTTPPipeline.
	Runtime struct {
		prev *HTTPPipeline
		curr *HTTPPipeline
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status struct {
		Timestamp uint64 `yaml:"timestamp"`

		Plugins map[string]interface{} `yaml:"plugins"`
	}
)

// InjectTimestamp injects timestamp.
func (s *Status) InjectTimestamp(t uint64) { s.Timestamp = t }

// NewRuntime creates an HTTPPipeline runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		prev: &HTTPPipeline{},
		curr: &HTTPPipeline{},
	}
}

func (r *Runtime) reload(next *HTTPPipeline) {
	for _, runningPlugin := range r.curr.runningPlugins {
		if next.getRunningPlugin(runningPlugin.meta.Name) == nil {
			if runningPlugin.pr.needClose {
				closer, ok := runningPlugin.plugin.(Closer)
				if ok {
					closer.Close()
				} else {
					logger.Errorf("BUG: %s got no Close",
						runningPlugin.pr.Kind)
				}
			}
		}
	}
	r.prev, r.curr = r.curr, next
}

// Status returns Status genreated by Runtime.
// NOTE: Caller must not call Status while reloading.
func (r *Runtime) Status() *Status {
	s := &Status{
		Plugins: make(map[string]interface{}),
	}

	for _, runningPlugin := range r.curr.runningPlugins {
		if runningPlugin.pr.needStatus {
			pluginStatus := reflect.ValueOf(runningPlugin.plugin).
				MethodByName("Status").Call(nil)[0].Interface()
			s.Plugins[runningPlugin.meta.Name] = pluginStatus
		}
	}

	return s
}

// Close closes runtime.
func (r *Runtime) Close() {
	r.reload(&HTTPPipeline{})
}
