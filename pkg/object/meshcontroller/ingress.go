package meshcontroller

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
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

// generate the mesh spec pipeline name
func genIngreePipelineSpecName(serviceName string) string {
	return fmt.Sprintf(meshServiceIngressPipelinePrefix, serviceName)
}

func genHTTPServerSpecName(serviceName string) string {

	return fmt.Sprintf(meshServiceIngressHTTPServerPrefix, serviceName)
}

type (
	IngressMsg struct {
		storeMsg storeOpMsg // original store notify operatons

		// for creating request
		serviceName  string
		instancePort uint32
	}

	// IngressServer control one ingress pipeline and one HTTPServer
	IngressServer struct {
		store MeshStorage

		// running EG objects, accept user traffic
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer

		mux sync.Mutex
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(store MeshStorage) *IngressServer {
	return &IngressServer{
		store:      store,
		Pipelines:  make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer: nil,
		mux:        sync.Mutex{},
	}
}

func (ings *IngressServer) HandleIngressOpMsg(msg IngressMsg) error {
	switch msg.storeMsg.op {
	case opTypeCreate:
		err := ings.createIngress(msg)
		return err

	case opTypeUpdate:
		err := ings.updateIngress(msg)
		return err

	default:
	}

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
