package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// IngressServer control one ingress pipeline and one HTTPServer
	IngressServer struct {
		store       storage.Storage
		super       *supervisor.Supervisor
		serviceName string
		port        uint32

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

func (ings *IngressServer) Get(name string) (protocol.HTTPHandler, bool) {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	p, ok := ings.Pipelines[name]
	return p, ok
}

// createIngress creates one default pipeline for ingress
func (ings *IngressServer) createIngress(server *spec.Service, port uint32) error {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	if _, ok := ings.Pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]; ok {
		// already been created
	} else {
		// gen ingress pipeline default spec
		pipelineSpec := server.GenDefaultIngressPipelineYAML(port)
		var pipeline httppipeline.HTTPPipeline
		superSpec, err := supervisor.NewSpec(pipelineSpec)
		if err != nil {
			logger.Errorf("BUG, gen ingress pipeline spec :%s , new super spec failed:%v", pipelineSpec, err)
			return err
		}
		pipeline.Init(superSpec, ings.super)
		ings.Pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)] = &pipeline
	}

	if ings.HTTPServer != nil {
		// already been created
	} else {
		var httpsvr httpserver.HTTPServer
		httpsvrSpec := server.GenDefaultIngressHTTPServerYAML()
		superSpec, err := supervisor.NewSpec(httpsvrSpec)
		if err != nil {
			logger.Errorf("BUG, gen ingress httpsvr spec :%s , new super spec failed:%v", httpsvrSpec, err)
			return err
		}
		httpsvr.Init(superSpec, ings.super)
		// for HTTPServer get running pipeline at traffic handing, since ingress server don't
		// call supervisior for creating HTTPServer and HTTPPipeline. Should find pipelines by
		// using ingress server itself.
		httpsvr.InjectMuxMapper(ings)
		ings.HTTPServer = &httpsvr
	}

	return nil
}

// UdpateIngressPipeline accepts new pipeline specs , and call it to update
// ingress's HTTPPipeline with inheritance
func (ings *IngressServer) UpdateIngressPipeline(newSpec string) error {
	var err error
	ings.mux.Lock()
	defer ings.mux.Unlock()
	pipeline, ok := ings.Pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]
	if !ok {
		return fmt.Errorf("ingress pipeline havn't been created yet")
	}

	superSpec, err := supervisor.NewSpec(newSpec)
	if err != nil {
		logger.Errorf("BUG, update ingress pipeline spec :%s , new super spec failed:%v", newSpec, err)
		return err
	}
	var newPipeline httppipeline.HTTPPipeline
	// safely close previous generation and create new pipeline
	newPipeline.Inherit(superSpec, pipeline, ings.super)
	ings.Pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)] = &newPipeline

	return err
}

//  CheckIngressReady checks ingress's pipeline and httpserver
//   are created or not
func (ings *IngressServer) CheckIngressReady() bool {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	_, pipelineReady := ings.Pipelines[spec.GenIngressPipelineObjectName(ings.serviceName)]

	return pipelineReady && (ings.HTTPServer != nil)
}
