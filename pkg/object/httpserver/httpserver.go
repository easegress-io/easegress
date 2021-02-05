package httpserver

import (
	"time"

	"github.com/megaease/easegateway/pkg/context"
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
		spec    *Spec
		runtime *runtime
	}

	// HTTPHandler is the common handler for the all backends
	// which handle the traffic from HTTPServer.
	HTTPHandler interface {
		Handle(ctx context.HTTPContext)
	}
)

// Category returns the category of HTTPServer.
func (hs *HTTPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of HTTPServer.
func (hs *HTTPServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HTTPServer.
func (hs *HTTPServer) DefaultSpec() supervisor.ObjectSpec {
	return &Spec{
		KeepAlive:        true,
		KeepAliveTimeout: "60s",
		MaxConnections:   10240,
	}
}

// Renew renews HTTPServer.
func (hs *HTTPServer) Renew(spec supervisor.ObjectSpec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	hs.spec = spec.(*Spec)

	if previousGeneration == nil {
		hs.runtime = newRuntime(super)
	} else {
		hs.runtime = previousGeneration.(*HTTPServer).runtime
	}

	hs.runtime.eventChan <- &eventReload{nextSpec: hs.spec}
}

// Handle is a dummy placeholder for supervisor.Object
func (hs *HTTPServer) Handle(context.HTTPContext) {}

// Status is the wrapper of runtime's Status.
func (hs *HTTPServer) Status() interface{} {
	if hs.runtime == nil {
		return &Status{}
	}

	return hs.runtime.Status()
}

// Close closes HTTPServer.
func (hs *HTTPServer) Close() {
	hs.runtime.Close()
}
