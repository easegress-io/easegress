package worker

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/service"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const egressRPCKey = "X-Mesh-Rpc-Service"

type (
	// EgressServer manages one/many ingress pipelines and one HTTPServer
	EgressServer struct {
		pipelines  map[string]*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer

		super       *supervisor.Supervisor
		serviceName string
		service     *service.Service
		mutex       sync.RWMutex
		watch       chan<- string
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName string, service *service.Service, watch chan<- string) *EgressServer {

	return &EgressServer{
		pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		serviceName: serviceName,
		service:     service,
		super:       super,
		watch:       watch,
	}
}

// Get gets egressServer itself as the default backend.
// egress server will handle the pipeline routing by itself.
func (egs *EgressServer) Get(name string) (protocol.HTTPHandler, bool) {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs, true
}

// CreateEgress creates a default Egress HTTPServer.
func (egs *EgressServer) CreateEgress(service *spec.Service) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if egs.httpServer == nil {
		var httpsvr httpserver.HTTPServer
		superSpec := service.EgressHTTPServerSpec()
		httpsvr.Init(superSpec, egs.super)
		httpsvr.InjectMuxMapper(egs)
		egs.httpServer = &httpsvr
	}
	return nil
}

// Ready checks Egress HTTPServer has been created or not.
// Not need to check pipelines, cause they will be dynamically added.
func (egs *EgressServer) Ready() bool {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs.httpServer != nil
}

func (egs *EgressServer) _getPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {

	pipeline, ok := egs.pipelines[serviceName]
	if ok {
		egs.watch <- serviceName
		return pipeline, nil
	}

	return nil, nil
}

func (egs *EgressServer) addPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {
	service := egs.service.GetServiceSpec(serviceName)

	instanceSpec := egs.service.ListServiceInstanceSpecs(serviceName)

	superSpec := service.EgressPipelineSpec(instanceSpec)

	pipeline := &httppipeline.HTTPPipeline{}
	pipeline.Init(superSpec, egs.super)
	egs.pipelines[service.Name] = pipeline

	return pipeline, nil
}

// DeletePipeline deletes one Egress pipeline according to the serviceName.
func (egs *EgressServer) DeletePipeline(serviceName string) {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if p, ok := egs.pipelines[serviceName]; ok {
		p.Close()
		delete(egs.pipelines, serviceName)
	}
}

// UpdatePipeline updates a local pipeline according to the informer.
func (egs *EgressServer) UpdatePipeline(service *spec.Service, instanceSpec []*spec.ServiceInstanceSpec) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	pipeline, err := egs._getPipeline(service.Name)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return fmt.Errorf("can't find service: %s's egress pipeline", service.Name)
	}

	newPipeline := &httppipeline.HTTPPipeline{}
	superSpec := service.EgressPipelineSpec(instanceSpec)
	newPipeline.Inherit(superSpec, pipeline, egs.super)
	egs.pipelines[service.Name] = newPipeline

	return nil
}

func (egs *EgressServer) getPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	pipeline, err := egs._getPipeline(serviceName)
	if err != nil {
		return nil, err
	} else {
		if pipeline != nil {
			return pipeline, nil
		}
	}

	pipeline, err = egs.addPipeline(serviceName)
	return pipeline, err
}

// Handle handles all egress traffic and route to desired pipeline according
// to the "X-MESH-RPC-SERVICE" field in header.
func (egs *EgressServer) Handle(ctx context.HTTPContext) {
	serviceName := ctx.Request().Header().Get(egressRPCKey)

	if len(serviceName) == 0 {
		logger.Errorf("handle egress RPC without setting service name in: %s header: %#v",
			egressRPCKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	logger.Infof("start get service name: %s", serviceName)
	pipeline, err := egs.getPipeline(serviceName)
	if err != nil {
		if err == spec.ErrServiceNotFound {
			logger.Errorf("handle egress RPC unknown service: %s", serviceName)
			ctx.Response().SetStatusCode(http.StatusNotFound)
		} else {
			logger.Errorf("handle egress RPC service: %s get pipeline failed: %v", serviceName, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
		}
		return
	}
	logger.Infof("get service name:%s pipeline:%#v", serviceName, pipeline)
	pipeline.Handle(ctx)
	logger.Infof("hanlde service name:%s finished, status code: %d", serviceName, ctx.Response().StatusCode())
}

// Close closes the Egress HTTPServer and Pipelines
func (egs *EgressServer) Close() {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	egs.httpServer.Close()
	for _, v := range egs.pipelines {
		v.Close()
	}
}
