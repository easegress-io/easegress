package worker

import (
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
)

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		store storage.Storage
		// running EG objects, accept user traffic
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(store storage.Storage) *IngressServer {
	return &IngressServer{
		store:      store,
		Pipelines:  make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer: nil,
	}
}

func (es *EgressServer) updateEgress(specs map[string]string) {

	return
}
