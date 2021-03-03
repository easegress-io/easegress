package httpserver

import (
	"time"

	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	// Category is the category of HTTPServer.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of HTTPServer.
	Kind = "HTTPServer"

	blockTimeout = 100 * time.Millisecond
)

func init() {
	supervisor.Register(&HTTPServer{})
}

type (
	// HTTPServer is Object HTTPServer.
	HTTPServer struct {
		runtime *runtime
	}

	// MuxMapper gets HTTP handler pipeline with mutex
	MuxMapper interface {
		Get(name string) (*supervisor.RunningObject, bool)
	}

	// SupervisorMapper calls supervisor for getting pipeline.
	SupervisorMapper struct {
		super *supervisor.Supervisor
	}
)

// Get gets pipeline from EG's running object
func (s *SupervisorMapper) Get(name string) (*supervisor.RunningObject, bool) {
	return s.super.GetRunningObject(name, supervisor.CategoryPipeline)
}

// Category returns the category of HTTPServer.
func (hs *HTTPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of HTTPServer.
func (hs *HTTPServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HTTPServer.
func (hs *HTTPServer) DefaultSpec() interface{} {
	return &Spec{
		KeepAlive:        true,
		KeepAliveTimeout: "60s",
		MaxConnections:   10240,
	}
}

// Init initilizes HTTPServer.
func (hs *HTTPServer) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	hs.runtime = newRuntime(super)

	hs.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		super:         super,
	}
}

// Inherit inherits previous generation of HTTPServer.
func (hs *HTTPServer) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	hs.runtime = previousGeneration.(*HTTPServer).runtime

	hs.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		super:         super,
	}
}

// Status is the wrapper of runtime's Status.
func (hs *HTTPServer) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: hs.runtime.Status(),
	}
}

// Close closes HTTPServer.
func (hs *HTTPServer) Close() {
	hs.runtime.Close()
}

// InjectMuxMapper inject a new mux mapper to route, it will cover the default map of supervisor.
func (hs *HTTPServer) InjectMuxMapper(mapper MuxMapper) {
	hs.runtime.SetMuxMapper(mapper)
}
