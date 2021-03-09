package worker

import (
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		// running EG objects, through Egress to visit other
		// service instances in mesh
		Pipelines  map[string]*httppipeline.HTTPPipeline
		HTTPServer *httpserver.HTTPServer

		super       *supervisor.Supervisor
		serviceName string
		mux         sync.RWMutex
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(super *supervisor.Supervisor, serviceName string) *IngressServer {
	return &IngressServer{
		Pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		HTTPServer:  nil,
		serviceName: serviceName,
		super:       super,
		mux:         sync.RWMutex{},
	}
}

// Get gets pipeline object for HTTPServer, it implements HTTPServer's MuxMapper interface
func (egs *EgressServer) Get(name string) (protocol.HTTPHandler, bool) {
	egs.mux.RLock()
	defer egs.mux.RUnlock()
	p, ok := egs.Pipelines[name]
	return p, ok
}

func (egs *EgressServer) createEgress() error {

	return nil
}

//
func (egs *EgressServer) createPipeline(reqServiceName string) error {
	// [TODO]
	return nil
}

func (egs *EgressServer) updatePipeline(reqServiceName string, newPipelineSpec string) error {
	// [TODO]
	return nil
}

func (egs *EgressServer) deletePipeline(reqServiceName string) error {
	// [TODO]
	return nil
}

func (egs *EgressServer) createHTTPServer(service *spec.Service) error {
	egs.mux.Lock()
	defer egs.mux.Unlock()
	var httpsvr httpserver.HTTPServer
	httpsvrSpec := service.GenDefaultEgressHTTPServerYAML()
	superSpec, err := supervisor.NewSpec(httpsvrSpec)
	if err != nil {
		logger.Errorf("BUG, gen egress httpsvr spec :%s , new super spec failed:%v", httpsvrSpec, err)
		return err
	}
	httpsvr.Init(superSpec, egs.super)
	httpsvr.InjectMuxMapper(egs)
	egs.HTTPServer = &httpsvr
	return nil
}

func (egs *EgressServer) updateEgress(specs map[string]string) {
	// [TODO]
	return
}
