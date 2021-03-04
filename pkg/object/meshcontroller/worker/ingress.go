package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/supervisor"
)

// genereate the EG running object name, which will be applied into
// memory
func genIngressPipelineObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-service-ingress-%s-pipeline", serviceName)
	return name
}
func genIngressHTTPSvrObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-service-ingress-%s-httpserver", serviceName)
	return name
}

type (
	IngressMsg struct {

		// for creating request
		serviceName  string
		instancePort uint32
	}

	// IngressServer control one ingress pipeline and one HTTPServer
	IngressServer struct {
		store storage.Storage
		super *supervisor.Supervisor

		// running EG objects, accept user traffic
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer

		mux sync.Mutex
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(store storage.Storage, super *supervisor.Supervisor) *IngressServer {
	return &IngressServer{
		store:      store,
		super:      super,
		Pipelines:  make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer: nil,
		mux:        sync.Mutex{},
	}
}

func (ings *IngressServer) HandleIngressOpMsg(msg IngressMsg) error {

	return nil
}

// createIngress creates one default pipeline and httpservice for ingress
func (ings *IngressServer) createIngress(msg IngressMsg) error {
	var err error
	// get ingress pipeline spec

	return err
}

func (ings *IngressServer) updateIngress(mgs IngressMsg) error {
	var err error

	return err
}

func (ings *IngressServer) deleteIngress() error {
	var err error

	return err
}
