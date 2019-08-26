package httpserver

import (
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/scheduler"
)

const (
	// Kind is HTTPServer kind.
	Kind = "HTTPServer"

	blockTimeout = 100 * time.Millisecond
)

func init() {
	scheduler.Register(&scheduler.ObjectRecord{
		Kind:              Kind,
		DefaultSpecFunc:   DefaultSpec,
		NewFunc:           New,
		DependObjectKinds: []string{},
	})
}

type (
	// HTTPServer is Object HTTPServer.
	HTTPServer struct {
		spec    *Spec
		runtime *runtime
	}
)

// DefaultSpec returns HTTPServer default spec.
func DefaultSpec() *Spec {
	return &Spec{
		KeepAlive:        true,
		KeepAliveTimeout: "60s",
		MaxConnections:   10240,
	}
}

// New creates an HTTPServer.
func New(spec *Spec, prev *HTTPServer, handlers *sync.Map) *HTTPServer {
	hs := &HTTPServer{
		spec: spec,
	}
	if hs.runtime == nil {
		hs.runtime = newRuntime(handlers)
	} else {
		hs.runtime = prev.runtime
	}

	hs.runtime.eventChan <- &eventReload{nextSpec: spec}

	return hs
}

// Handle is a dummy placeholder for scheduler.Object
func (hs *HTTPServer) Handle(context.HTTPContext) {}

// Status is the wrapper of runtime's Status.
func (hs *HTTPServer) Status() *Status {
	return hs.runtime.Status()
}

// Close closes HTTPServer.
// Nothing to do.
func (hs *HTTPServer) Close() {}
