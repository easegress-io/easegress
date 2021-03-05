package worker

import (
	"sync"

	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
)

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		// running EG objects, accept user traffic
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer

		mux sync.RWMutex
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer() *IngressServer {
	return &IngressServer{
		Pipelines:  make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer: nil,
		mux:        sync.RWMutex{},
	}
}

func (es *EgressServer) updateEgress(specs map[string]string) {

	return
}
