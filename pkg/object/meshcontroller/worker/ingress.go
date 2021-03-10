package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// IngressServer manages one ingress pipeline and one HTTPServer
	IngressServer struct {
		super       *supervisor.Supervisor
		serviceName string
		//Note: this is the Java business process's listening port
		//      not the ingress HTTPServer's port
		port uint32
		mux  sync.RWMutex

		// running EG objects, accept other service instances' traffic
		// in mesh and hand over to local Java business process
		pipelines  map[string]*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(super *supervisor.Supervisor, serviceName string) *IngressServer {
	return &IngressServer{
		super:       super,
		pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		httpServer:  nil,
		serviceName: serviceName,
		mux:         sync.RWMutex{},
	}
}

// Get gets pipeline object for httpServer, it implements httpServer's MuxMapper interface
func (ings *IngressServer) Get(name string) (protocol.HTTPHandler, bool) {
	ings.mux.RLock()
	defer ings.mux.RUnlock()
	p, ok := ings.pipelines[name]
	return p, ok
}

// Ready checks ingress's pipeline and httpServer
// are created or not
func (ings *IngressServer) Ready() bool {
	ings.mux.RLock()
	defer ings.mux.RUnlock()
	_, pipelineReady := ings.pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]

	return pipelineReady && (ings.httpServer != nil)
}

// createIngress creates local default pipeline and httpServer for ingress
func (ings *IngressServer) createIngress(service *spec.Service, port uint32) error {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	if _, ok := ings.pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]; ok {
		// already been created
	} else {
		// gen ingress pipeline default spec
		pipelineSpec := service.GenDefaultIngressPipelineYAML(port)
		var pipeline httppipeline.HTTPPipeline
		superSpec, err := supervisor.NewSpec(pipelineSpec)
		if err != nil {
			logger.Errorf("BUG, gen ingress pipeline spec :%s , new super spec failed:%v", pipelineSpec, err)
			return err
		}
		pipeline.Init(superSpec, ings.super)
		ings.pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)] = &pipeline
	}

	if ings.httpServer != nil {
		// already been created
	} else {
		var httpsvr httpserver.HTTPServer
		httpsvrSpec := service.GenDefaultIngressHTTPServerYAML()
		superSpec, err := supervisor.NewSpec(httpsvrSpec)
		if err != nil {
			logger.Errorf("BUG, gen ingress httpsvr spec :%s , new super spec failed:%v", httpsvrSpec, err)
			return err
		}
		httpsvr.Init(superSpec, ings.super)
		// for httpServer get running pipeline at traffic handing, since ingress server don't
		// call supervisior for creating httpServer and HTTPPipeline. Should find pipelines by
		// using ingress server itself.
		httpsvr.InjectMuxMapper(ings)
		ings.httpServer = &httpsvr
	}

	return nil
}

// UpdatePipeline accepts new pipeline specs, and uses it to update
// ingress's HTTPPipeline with inheritance
func (ings *IngressServer) UpdatePipeline(newSpec string) error {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	pipeline, ok := ings.pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]
	if !ok {
		return fmt.Errorf("service :%s's ingress pipeline havn't been created yet", ings.serviceName)
	}

	superSpec, err := supervisor.NewSpec(newSpec)
	if err != nil {
		logger.Errorf("BUG, update ingress pipeline spec :%s , new super spec failed:%v", newSpec, err)
		return err
	}
	var newPipeline httppipeline.HTTPPipeline
	// safely close previous generation and create new pipeline
	newPipeline.Inherit(superSpec, pipeline, ings.super)
	ings.pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)] = &newPipeline

	return err
}
