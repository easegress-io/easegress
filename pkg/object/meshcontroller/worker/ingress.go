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

const (
	defaultIngressSchema = "http"
	defaultIngressIP     = "127.0.0.1"
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

// The default yaml config of ingress pipeline
var (
	defaultIngressPipeline = `
name: %s 
kind: HTTPPipeline
flow:
  - filter: %s 
filters:
  - name: %s 
    kind: Backend
    mainPool:
      servers:
      - url: %s 
`
	defaultIngressHTTPServer = `
kind: HTTPServer
name: %s 
port: %d 
rules:
  - paths:
    - path: /
      backend: %s, 
`
)

// genereate the EG running object name, which will be applied into
// memory
func genIngressPipelineObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-pipeline", serviceName)
	return name
}

func genIngressBackendFilterName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-backend", serviceName)
	return name
}

func genIngressHTTPSvrObjectName(serviceName string) string {
	name := fmt.Sprintf("mesh-ingress-%s-httpserver", serviceName)
	return name
}

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
func (ings *IngressServer) createIngress(sidecar *spec.Sidecar) error {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	if _, ok := ings.Pipelines[genIngressPipelineObjectName(ings.serviceName)]; ok {
		// already been created
	} else {
		// get ingress pipeline spec
		addr := fmt.Sprintf("%s://%s:%d", defaultIngressSchema, defaultIngressIP, ings.port)

		pipelineSpec := fmt.Sprintf(defaultIngressPipeline, genIngressPipelineObjectName(ings.serviceName),
			genIngressBackendFilterName(ings.serviceName),
			genIngressBackendFilterName(ings.serviceName),
			addr)

		var pipeline httppipeline.HTTPPipeline
		superSpec, err := supervisor.NewSpec(pipelineSpec)
		if err != nil {
			logger.Errorf("BUG, gen ingress pipeline spec :%s , new super spec failed:%v", pipelineSpec, err)
			return err
		}

		pipeline.Init(superSpec, ings.super)
		ings.Pipelines[genIngressPipelineObjectName(ings.serviceName)] = &pipeline
	}

	if ings.HTTPServer != nil {
		// already been created
	} else {
		httpsvrSpec := fmt.Sprintf(defaultIngressHTTPServer, genIngressHTTPSvrObjectName(ings.serviceName),
			sidecar.IngressPort, genIngressPipelineObjectName(ings.serviceName))
		var httpsvr httpserver.HTTPServer
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

// UdpateIngressPipeline accepts PipelineUpdater, and call it to update
// ingress's HTTPPipeline with inheritance
func (ings *IngressServer) UpdateIngressPipeline(newSpec string) error {
	var err error
	ings.mux.Lock()
	defer ings.mux.Unlock()
	pipeline, ok := ings.Pipelines[genIngressPipelineObjectName(ings.serviceName)]
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

	ings.Pipelines[genIngressPipelineObjectName(ings.serviceName)] = &newPipeline

	return err
}

//  CheckIngressReady checks ingress's pipeline and httpserver
//   are created or not
func (ings *IngressServer) CheckIngressReady() bool {
	ings.mux.Lock()
	defer ings.mux.Unlock()
	_, pipelineReady := ings.Pipelines[genIngressPipelineObjectName(ings.serviceName)]

	return pipelineReady && (ings.HTTPServer != nil)
}
