package worker

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

var errEgressHTTPServerNotExist = fmt.Errorf("egress http server not exist")

const egressRPCKey = "mesh_rpc_service"

type (
	// EgressServer handle egress traffic gate
	EgressServer struct {
		// running EG objects, through Egress to visit other
		// service instances in mesh
		pipelines  map[string]*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer

		super       *supervisor.Supervisor
		serviceName string
		store       storage.Storage
		mux         sync.RWMutex
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(super *supervisor.Supervisor, serviceName string, store storage.Storage) *EgressServer {
	return &EgressServer{
		pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		httpServer:  nil,
		serviceName: serviceName,
		store:       store,
		super:       super,
		mux:         sync.RWMutex{},
	}
}

// Get gets egressServer itself as the default backend
// egress server will handle the pipeline routing inside
func (egs *EgressServer) Get(name string) (protocol.HTTPHandler, bool) {
	return egs, true
}

func (egs *EgressServer) createEgress(service *spec.Service) error {
	egs.mux.Lock()
	defer egs.mux.Unlock()

	if egs.httpServer == nil {
		if err := egs.createHTTPServer(service); err != nil {
			return err
		}
	}
	return nil
}

// Ready checks Egress HTTPServer has been created or not.
// Pipeline will dynamicly added into Egress
func (egs *EgressServer) Ready() bool {
	egs.mux.RLock()
	defer egs.mux.RUnlock()
	return egs.httpServer != nil
}

func (egs *EgressServer) addEgress(service *spec.Service, ins []*spec.ServiceInstance) error {
	if service.Name == egs.serviceName {
		return nil
	}

	if egs.httpServer == nil {
		logger.Errorf("egress, add one service :%s before create egress successfully", service.Name)
		return errEgressHTTPServerNotExist
	}

	if egs.pipelines[service.Name] == nil {
		if err := egs.createPipeline(service, ins); err != nil {
			return err
		}
	}

	return nil
}

//
func (egs *EgressServer) createPipeline(service *spec.Service, ins []*spec.ServiceInstance) error {
	var pipeline httppipeline.HTTPPipeline
	superSpec, err := service.ToEgressPipelineSpec(ins)
	if err != nil {
		return err
	}

	pipeline.Init(superSpec, egs.super)
	egs.pipelines[service.Name] = &pipeline
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
	egs.httpServer = &httpsvr
	return nil
}

// Handle handles all egress traffic and route to desired
// pipeline according to the "mesh_rpc_service" field in header
func (egs *EgressServer) Handle(ctx context.HTTPContext) {
	serviceName := ctx.Request().Header().Get(egressRPCKey)

	if len(serviceName) == 0 {
		logger.Errorf("egress handle rpc without setting serivce name in %s, header:%#v",
			egressRPCKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	egs.mux.Lock()
	pipeline, ok := egs.pipelines[serviceName]

	// create one pipeline
	if !ok {
		service, err := getService(serviceName, egs.store)
		if err != nil {
			egs.mux.Unlock()
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
			return
		}
		ins, err := getSerivceInstances(serviceName, egs.store)
		if err != nil {
			egs.mux.Unlock()
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
			return
		}
		if err = egs.addEgress(service, ins); err != nil {
			egs.mux.Unlock()
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
			return
		}
		pipeline = egs.pipelines[serviceName]
	}

	egs.mux.Unlock()
	pipeline.Handle(ctx)

	return
}
