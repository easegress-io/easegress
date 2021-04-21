package ingresscontroller

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/service"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// IngressController is the ingress controller.
	IngressController struct {
		mutex sync.RWMutex

		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin

		service   *service.Service
		pipelines map[string]*httppipeline.HTTPPipeline
		httpSvr   *httpserver.HTTPServer
		informer  informer.Informer

		addServiceEvent    chan string
		removeServiceEvent chan string
		done               chan struct{}
	}
)

const (
	serviceEventChanSize = 100
)

// New creates a mesh ingress controller.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *IngressController {
	store := storage.New(superSpec.Name(), super.Cluster())

	ic := &IngressController{
		super:     super,
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),

		service:   service.New(superSpec, store),
		pipelines: make(map[string]*httppipeline.HTTPPipeline),
		informer:  informer.NewInformer(store),

		addServiceEvent:    make(chan string, serviceEventChanSize),
		removeServiceEvent: make(chan string, serviceEventChanSize),
		done:               make(chan struct{}),
	}

	ic.initHTTPServer()

	go ic.watchEvent()
	return ic
}

func (ic *IngressController) initHTTPServer() {
	var rules []*spec.IngressRule
	ingresses := ic.service.ListIngressSpecs()
	for _, ingress := range ingresses {
		for i := range ingress.Rules {
			rules = append(rules, &ingress.Rules[i])
		}
	}

	spec := spec.IngressHTTPServerSpec(ic.spec.IngressPort, rules)
	ic.updateHTTPServer(spec)
}

func (ic *IngressController) updateHTTPServer(spec *supervisor.Spec) {
	httpSvr := httpserver.HTTPServer{}
	if ic.httpSvr == nil {
		httpSvr.Init(spec, ic.super)
	} else {
		httpSvr.Inherit(spec, ic.httpSvr, ic.super)
	}
	httpSvr.InjectMuxMapper(ic)
	ic.httpSvr = &httpSvr
}

func (ic *IngressController) addPipeline(serviceName string) (*httppipeline.HTTPPipeline, error) {
	service := ic.service.GetServiceSpec(serviceName)
	if service == nil {
		return nil, spec.ErrServiceNotFound
	}

	instanceSpec := ic.service.ListServiceInstanceSpecs(serviceName)
	if len(instanceSpec) == 0 {
		logger.Errorf("found service: %s with empty instances", serviceName)
		return nil, spec.ErrServiceNotavailable
	}

	superSpec := service.IngressPipelineSpec(instanceSpec)
	logger.Infof("add pipeline spec: %s", superSpec.YAMLConfig())

	pipeline := &httppipeline.HTTPPipeline{}
	pipeline.Init(superSpec, ic.super)
	ic.pipelines[serviceName] = pipeline

	return pipeline, nil
}

func (ic *IngressController) deletePipeline(serviceName string) {
	ic.mutex.Lock()

	p := ic.pipelines[serviceName]
	if p != nil {
		delete(ic.pipelines, serviceName)
	}

	ic.mutex.Unlock()

	if p != nil {
		p.Close()
	}
}

func (ic *IngressController) updatePipeline(
	service *spec.Service,
	instanceSpec []*spec.ServiceInstanceSpec,
) error {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	pipeline, ok := ic.pipelines[service.Name]
	if !ok {
		return fmt.Errorf("BUG: can't find service: %s's pipeline", service.Name)
	}

	newPipeline := &httppipeline.HTTPPipeline{}
	superSpec := service.IngressPipelineSpec(instanceSpec)
	newPipeline.Inherit(superSpec, pipeline, ic.super)
	ic.pipelines[service.Name] = newPipeline

	return nil
}

// Get gets pipe line for backend 'name'
func (ic *IngressController) Get(name string) (protocol.HTTPHandler, bool) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	pipeline, ok := ic.pipelines[name]
	if ok {
		ic.addServiceEvent <- name
		return pipeline, true
	}

	// BUG? the pipeline will be added again and again
	// if service named 'name' does not exist
	pipeline, err := ic.addPipeline(name)
	if err == nil {
		ic.addServiceEvent <- name
	}

	return pipeline, err == nil
}

func (ic *IngressController) recover() {
	if err := recover(); err != nil {
		const format = "%s: recover from: %v, stack trace:\n%s\n"
		logger.Errorf(format, ic.superSpec.Name(), err, debug.Stack())
	}
}

func (ic *IngressController) watchIngress() {
	handler := func(ingresses map[string]*spec.Ingress) bool {
		defer ic.recover()

		logger.Infof("handle informer ingress update event: %#v", ingresses)

		services := make(map[string]bool)
		var rules []*spec.IngressRule
		for _, ingress := range ingresses {
			for i := range ingress.Rules {
				r := &ingress.Rules[i]
				rules = append(rules, r)
				for j := range r.Paths {
					p := &r.Paths[j]
					services[p.Backend] = true
				}
			}
		}

		spec := spec.IngressHTTPServerSpec(ic.spec.IngressPort, rules)

		ic.mutex.Lock()
		defer ic.mutex.Unlock()

		ic.updateHTTPServer(spec)
		for name := range ic.pipelines {
			if !services[name] {
				ic.removeServiceEvent <- name
			}
		}

		return true
	}

	err := ic.informer.OnIngressSpecs(handler)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add scope watching ingress failed: %v", err)
	}
}

func (ic *IngressController) stopWatchService(name string) {
	logger.Infof("stop watching service %s as it is removed from all ingress rules", name)
	ic.informer.StopWatchServiceSpec(name, informer.AllParts)
	ic.informer.StopWatchServiceInstanceSpec(name)
}

func (ic *IngressController) watchService(name string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) bool {
		defer ic.recover()

		switch event.EventType {
		case informer.EventDelete:
			logger.Infof("handle informer service: %s's spec delete event", name)
			ic.deletePipeline(name)
			return false

		case informer.EventUpdate:
			logger.Infof("handle informer service: %s's spec update event", name)
			instanceSpecs := ic.service.ListServiceInstanceSpecs(service.Name)
			if err := ic.updatePipeline(service, instanceSpecs); err != nil {
				logger.Errorf("handle informer update service: %s's failed: %v", name, err)
			}
		}

		return true
	}

	err := ic.informer.OnPartOfServiceSpec(name, informer.AllParts, handleSerivceSpec)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add scope watching service: %s failed: %v", name, err)
		return
	}

	handleServiceInstances := func(instanceKvs map[string]*spec.ServiceInstanceSpec) bool {
		defer ic.recover()

		logger.Infof("handle informer service: %s's instance update event, ins: %#v", name, instanceKvs)
		serviceSpec := ic.service.GetServiceSpec(name)

		var instanceSpecs []*spec.ServiceInstanceSpec
		for _, v := range instanceKvs {
			instanceSpecs = append(instanceSpecs, v)
		}
		if err := ic.updatePipeline(serviceSpec, instanceSpecs); err != nil {
			logger.Errorf("handle informer failed, update service: %s failed: %v", name, err)
		}

		return true
	}

	err = ic.informer.OnServiceInstanceSpecs(name, handleServiceInstances)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add prefix watching service: %s failed: %v", name, err)
		return
	}
}

func (ic *IngressController) watchEvent() {
	ic.watchIngress()

	for {
		select {
		case <-ic.done:
			return
		case name := <-ic.addServiceEvent:
			ic.watchService(name)
		case name := <-ic.removeServiceEvent:
			ic.stopWatchService(name)
			ic.deletePipeline(name)
		}
	}
}

// Status returns the status of IngressController.
func (ic *IngressController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

func (ic *IngressController) Close() {
	close(ic.done)

	ic.informer.Close()
	ic.httpSvr.Close()
}
