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

const egressRPCKey = "X-MESH-RPC-SERVICE"

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

// CreateEgress creates a default Egress HTTPServer
func (egs *EgressServer) CreateEgress(service *spec.Service) error {
	egs.mux.Lock()
	defer egs.mux.Unlock()

	if egs.httpServer == nil {
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

func (egs *EgressServer) addPipeline(service *spec.Service, ins []*spec.ServiceInstance) error {
	if egs.httpServer == nil {
		logger.Errorf("egress, add one service :%s before create egress successfully", service.Name)
		return errEgressHTTPServerNotExist
	}

	if egs.pipelines[service.Name] == nil {
		var pipeline httppipeline.HTTPPipeline
		superSpec, err := service.ToEgressPipelineSpec(ins)
		if err != nil {
			logger.Errorf("serivce :%#v, with instances :%#v create pipeline spec failed,err:%v ",
				service, ins, err)
			return err
		}

		pipeline.Init(superSpec, egs.super)
		egs.pipelines[service.Name] = &pipeline
	}
	return nil
}

// UpdatePipeline updates a local pipeline according to the informer
func (egs *EgressServer) UpdatePipeline(service *spec.Service, ins []*spec.ServiceInstance) error {
	// [TODO]
	return nil
}

// GetPipeline gets one local pipeline, if it not exist locally but can be discovried,
// create it.
func (egs *EgressServer) GetPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {
	egs.mux.RLock()
	pipeline, ok := egs.pipelines[serviceName]
	egs.mux.RUnlock()
	if ok {
		return pipeline, nil
	}

	egs.mux.Lock()
	defer egs.mux.Unlock()
	// double check, in case other goroutine has
	// create the pipeline already
	pipeline, ok := egs.pipelines[serviceName]
	if ok {
		return pipeline, nil
	}

	// create one pipeline
	service, err := getService(serviceName, egs.store)
	if err != nil {
		return nil, err
	}

	ins, err := getSerivceInstances(serviceName, egs.store)
	if err != nil {
		return nil, err
	}
	if err = egs.addPipeline(service, ins); err != nil {
		return nil, err
	}
	pipeline = egs.pipelines[serviceName]

	return pipeline, nil
}

// Handle handles all egress traffic and route to desired
// pipeline according to the "X-MESH-RPC-SERVICE" field in header
func (egs *EgressServer) Handle(ctx context.HTTPContext) {
	serviceName := ctx.Request().Header().Get(egressRPCKey)

	if len(serviceName) == 0 {
		logger.Errorf("egress handle rpc without setting serivce name in %s, header:%#v",
			egressRPCKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	pipeline, err := egs.GetPipeline(serviceName)
	if err != nil {
		if err == spec.ErrServiceNotFound {
			logger.Errorf("egress handle rpc unknow service:%s", serviceName)
			ctx.Response().SetStatusCode(http.StatusNotFound)
		} else {
			logger.Errorf("egress handle rpc service:%s, get pipeline failed:%v", serviceName, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
		}
		return
	}

	pipeline.Handle(ctx)

	return
}
