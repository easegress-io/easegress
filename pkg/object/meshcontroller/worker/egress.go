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
	"gopkg.in/yaml.v2"
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
		// update this spec, then apply into httpServer
		httpServerSpec *httpserver.Spec

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
	p, ok := egs.pipelines[name]
	return p, ok
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

	egs.mux.Lock()
	defer egs.mux.Unlock()

	var err error
	if egs.httpServer == nil {
		logger.Errorf("egress, add one service :%s before create egress successfully", service.Name)
		return errEgressHTTPServerNotExist
	}

	if egs.pipelines[service.Name] == nil {
		if err = egs.createPipeline(service, ins); err != nil {
			return err
		}
	}

	found := false
	for _, v := range egs.httpServerSpec.Rules {
		for _, p := range v.Paths {
			for _, h := range p.Headers {
				if h.Backend == service.Name && h.Key == egressRPCKey {
					for _, hv := range h.Values {
						if hv == service.Name {
							found = true
							break
						}
					}
				}
			}
		}
	}

	if !found {
		rule := httpserver.Rule{
			Paths: []httpserver.Path{
				{
					Headers: []*httpserver.Header{
						{
							Backend: service.Name,
							Key:     egressRPCKey,
							Values:  []string{service.Name},
						},
					},
				},
			},
		}
		originSpec := egs.httpServerSpec
		egs.httpServerSpec.Rules = append(egs.httpServerSpec.Rules, rule)
		if err = egs.updateHTTPServer(); err != nil {
			// rollback
			egs.httpServerSpec = originSpec
			return err
		}
	}
	return err
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
	egs.httpServer = &httpsvr
	egs.httpServerSpec = superSpec.ObjectSpec().(*httpserver.Spec)
	return nil
}

func (egs *EgressServer) updateHTTPServer() error {
	spec, err := yaml.Marshal(egs.httpServerSpec)
	if err != nil {
		logger.Errorf("BUG, yaml marshal egress httpserver:%v failed, err :%v", egs.httpServerSpec, err)
		return err
	}

	superSpec, err := supervisor.NewSpec(string(spec))
	if err != nil {
		logger.Errorf("BUG, update egress httpserver spec :%v , new super spec failed:%v", string(spec), err)
		return err
	}

	var newHTTPserver httpserver.HTTPServer
	newHTTPserver.Inherit(superSpec, egs.httpServer, egs.super)
	egs.httpServer = &newHTTPserver
	return nil
}
