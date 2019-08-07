package httpserver

import (
	"time"

	"github.com/megaease/easegateway/pkg/registry"
)

func init() {
	registry.Register(Kind, DefaultSpec)
}

const (
	// Kind is HTTPServer kind.
	Kind = "HTTPServer"

	blockTimeout = 100 * time.Millisecond
)

type (
	// HTTPServer is Object HTTPServer.
	HTTPServer struct {
		spec    *Spec
		runtime *Runtime
	}
)

// DefaultSpec returns HTTPServer default spec.
func DefaultSpec() registry.Spec {
	return &Spec{
		KeepAlive:        true,
		KeepAliveTimeout: "60s",
		MaxConnections:   10240,
	}
}

// New creates an HTTPServer.
func New(spec *Spec, runtime *Runtime, blockToReady bool) *HTTPServer {
	hs := &HTTPServer{
		spec:    spec,
		runtime: runtime,
	}

	runtime.eventChan <- &eventReload{nextSpec: spec}

	if blockToReady {
		for {
			time.Sleep(blockTimeout)
			if runtime.Status().State == stateRunning {
				break
			}
		}
	}

	return hs
}

// Close closes HTTPServer.
// Nothing to do.
func (hs *HTTPServer) Close() {}

// Kind returns HTTPServer.
func (hs *HTTPServer) Kind() string { return "HTTPServer" }
