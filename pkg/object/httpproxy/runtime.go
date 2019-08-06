package httpproxy

import (
	"github.com/megaease/easegateway/pkg/object/httppipeline"
)

type (

	// Runtime contains all runtime info of HTTPProxy.
	Runtime struct {
		pipeline *httppipeline.Runtime
	}

	// Status contains all status gernerated by runtime, for displaying to users.
	Status map[string]interface{}
)

// InjectTimestamp injects timestamp.
func (s *Status) InjectTimestamp(t uint64) { (*s)["timestamp"] = t }

// NewRuntime creates an HTTPProxy runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		pipeline: httppipeline.NewRuntime(),
	}
}

// Status returns Status genreated by Runtime.
// NOTE: Caller must not call Status while reloading.
func (r *Runtime) Status() *Status {
	s := Status(r.pipeline.Status().Plugins)
	return &s
}

// Close closes runtime.
func (r *Runtime) Close() {
	r.pipeline.Close()
}
