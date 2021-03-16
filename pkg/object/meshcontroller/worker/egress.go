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
	// EgressServer manages one/many ingress pipelines and one HTTPServer
	EgressServer struct {
		pipelines  map[string]*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer

		super       *supervisor.Supervisor
		serviceName string
		store       storage.Storage
		mutex       sync.RWMutex
		watch       chan<- string
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(super *supervisor.Supervisor, serviceName string, store storage.Storage, watch chan<- string) *EgressServer {
	return &EgressServer{
		pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		httpServer:  nil,
		serviceName: serviceName,
		store:       store,
		super:       super,
		mutex:       sync.RWMutex{},
		watch:       watch,
	}
}

// Get gets egressServer itself as the default backend.
// egress server will handle the pipeline routing by itself.
func (egs *EgressServer) Get(name string) (protocol.HTTPHandler, bool) {
	return egs, true
}

// CreateEgress creates a default Egress HTTPServer.
func (egs *EgressServer) CreateEgress(service *spec.Service) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

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
// Not need to check pipelines, cause they will be dynamicly added.
func (egs *EgressServer) Ready() bool {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs.httpServer != nil
}

func (egs *EgressServer) addPipeline(service *spec.Service, ins []*spec.ServiceInstanceSpec) error {
	if egs.httpServer == nil {
		logger.Errorf("add one service :%s before create egress successfully", service.Name)
		return errEgressHTTPServerNotExist
	}

	if egs.pipelines[service.Name] == nil {
		var pipeline httppipeline.HTTPPipeline
		superSpec, err := service.ToEgressPipelineSpec(ins)
		if err != nil {
			logger.Errorf("to egress pipeline spec failed, serivce :%#v, with instances :%#v ,err:%v ",
				service, ins, err)
			return err
		}

		pipeline.Init(superSpec, egs.super)
		egs.pipelines[service.Name] = &pipeline
	}
	return nil
}

// DeletePipeline deletes one Egress pipeline accoring to the serviceName.
func (egs *EgressServer) DeletePipeline(serviceName string) {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	delete(egs.pipelines, serviceName)

	return
}

// UpdatePipeline updates a local pipeline according to the informer.
func (egs *EgressServer) UpdatePipeline(service *spec.Service, ins []*spec.ServiceInstanceSpec) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	pipeline, ok := egs.pipelines[service.Name]
	if !ok {
		return fmt.Errorf("can't find service:%s's egress pipeline", service.Name)
	}

	superSpec, err := service.ToEgressPipelineSpec(ins)
	if err != nil {
		logger.Errorf("BUG: update egress pipeline servcie:%#v ,ins:%#v, failed:%v", service, ins, err)
		return err
	}
	var newPipeline httppipeline.HTTPPipeline
	newPipeline.Inherit(superSpec, pipeline, egs.super)
	egs.pipelines[service.Name] = &newPipeline

	return nil
}

func (egs *EgressServer) getPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {
	egs.mutex.RLock()
	pipeline, ok := egs.pipelines[serviceName]
	egs.mutex.RUnlock()
	if ok {
		egs.watch <- serviceName
		return pipeline, nil
	}

	egs.mutex.Lock()
	defer egs.mutex.Unlock()
	// double check, in case other goroutine has
	// created the pipeline already
	pipeline, ok = egs.pipelines[serviceName]
	if ok {
		egs.watch <- serviceName
		return pipeline, nil
	}

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
	egs.watch <- serviceName
	return pipeline, nil
}

// Handle handles all egress traffic and route to desired pipeline according
// to the "X-MESH-RPC-SERVICE" field in header.
func (egs *EgressServer) Handle(ctx context.HTTPContext) {
	serviceName := ctx.Request().Header().Get(egressRPCKey)

	if len(serviceName) == 0 {
		logger.Errorf("handle egress rpc without setting serivce name in %s, header:%#v",
			egressRPCKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	pipeline, err := egs.getPipeline(serviceName)
	if err != nil {
		if err == spec.ErrServiceNotFound {
			logger.Errorf("handle egress rpc unknow service:%s", serviceName)
			ctx.Response().SetStatusCode(http.StatusNotFound)
		} else {
			logger.Errorf("handle egress rpc service:%s, get pipeline failed:%v", serviceName, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
		}
		return
	}

	pipeline.Handle(ctx)

	return
}
