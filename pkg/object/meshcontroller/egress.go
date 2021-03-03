package meshcontroller

import (
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
)

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		store MeshStorage
		// running EG objects, accept user traffic
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer
	}

	// EngressMsg is the engress notify message wrappered with basic
	// store message
	EngressMsg struct {
		storeMsg storeOpMsg // original store notify operatons
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(store MeshStorage) *IngressServer {
	return &IngressServer{
		store:      store,
		Pipelines:  make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer: nil,
	}
}

func (es *EgressServer) updateEgress(specs map[string]string) {

	return
}
