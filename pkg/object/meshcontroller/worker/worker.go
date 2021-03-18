package worker

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/service"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/supervisor"
	"gopkg.in/yaml.v2"
)

type (
	// Worker is a sidecar in service mesh.
	Worker struct {
		mutex sync.Mutex

		super             *supervisor.Supervisor
		superSpec         *supervisor.Spec
		spec              *spec.Admin
		heartbeatInterval time.Duration

		// mesh service fields
		serviceName     string
		instanceID      string
		aliveProbe      string
		applicationPort uint32
		applicationIP   string

		store    storage.Storage
		service  *service.Service
		informer informer.Informer

		registryServer       *registrycenter.Server
		ingressServer        *IngressServer
		egressServer         *EgressServer
		observabilityManager *ObservabilityManager

		egressEvent chan string
		done        chan struct{}
	}
)

const (
	egressEventChanSize = 100

	labelApplicationPort = "application-port"
	labelAliveProbe      = "alive-probe"
	labelServiceName     = "mesh-servicename"

	// from k8s pod's env value
	podEnvHostname      = "HOSTNAME"
	podEnvApplicationIP = "APPLICATION_IP"
)

// New creates a mesh worker.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)

	serviceName := option.Global.Labels[labelServiceName]
	aliveProbe := option.Global.Labels[labelAliveProbe]
	applicationPort, err := strconv.Atoi(option.Global.Labels[labelApplicationPort])
	if err != nil {
		logger.Errorf("parse %s failed: %v", labelApplicationPort, err)
	}

	instanceID := os.Getenv(podEnvHostname)
	applicationIP := os.Getenv(podEnvApplicationIP)

	store := storage.New(superSpec.Name(), super.Cluster())
	_service := service.New(superSpec, store)
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, applicationIP, applicationPort, instanceID, _service)
	ingressServer := NewIngressServer(super, serviceName)
	egressEvent := make(chan string, egressEventChanSize)
	egressServer := NewEgressServer(superSpec, super, serviceName, store, egressEvent)
	observabilityManager := NewObservabilityServer(serviceName)
	inf := informer.NewInformer(store)

	w := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,

		serviceName:     serviceName,
		instanceID:      instanceID, // instanceID will be the port ID
		aliveProbe:      aliveProbe,
		applicationPort: uint32(applicationPort),
		applicationIP:   applicationIP,

		store:    store,
		service:  _service,
		informer: inf,

		registryServer:       registryCenterServer,
		ingressServer:        ingressServer,
		egressServer:         egressServer,
		observabilityManager: observabilityManager,

		egressEvent: egressEvent,
		done:        make(chan struct{}),
	}

	w.registerAPIs()

	go w.run()

	return w
}

func (w *Worker) run() {
	var err error
	w.heartbeatInterval, err = time.ParseDuration(w.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval %s failed: %v",
			w.spec.HeartbeatInterval, err)
		return
	}

	if len(w.serviceName) == 0 {
		logger.Errorf("mesh service name is empty")
		return
	} else {
		logger.Infof("%s works for service %s", w.serviceName)
	}

	_, err = url.ParseRequestURI(w.aliveProbe)
	if err != nil {
		logger.Errorf("parse alive probe %s to url failed: %v", w.aliveProbe, err)
		return
	}

	if w.applicationPort == 0 {
		logger.Errorf("empty application port")
		return
	}

	if len(w.instanceID) == 0 {
		logger.Errorf("empty env HOST ")
		return
	}

	if len(w.applicationIP) == 0 {
		logger.Errorf("empty env APPLICTION IP")
		return
	}

	// asynchronous register itself
	w.register()
	go w.heartbeat()
	go w.watchEvent()
}

func (w *Worker) register() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
				w.superSpec.Name(), err, debug.Stack())
		}
	}()

	serviceSpec := w.service.GetServiceSpec(w.serviceName)
	if serviceSpec == nil {
		logger.Errorf("registry to unknown service %s", w.serviceName)
		return
	}

	w.registryServer.Register(serviceSpec, w.ingressServer.Ready, w.egressServer.Ready)
}

func (w *Worker) heartbeat() {
	observabilityReady, trafficGateReady := false, false

	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					w.superSpec.Name(), err, debug.Stack())
			}
		}()

		if !trafficGateReady {
			err := w.initTrafficGate()
			if err != nil {
				logger.Errorf("init traffic gate failed: %v", err)
			} else {
				trafficGateReady = true
			}
		}

		if w.registryServer.Registered() {
			if !observabilityReady {
				err := w.informObservability()
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					observabilityReady = true
				}
			}

			err := w.updateHearbeat()
			if err != nil {
				logger.Errorf("update heartbeart failed: %v", err)
			}
		}
	}

	for {
		select {
		case <-w.done:
			return
		case <-time.After(w.heartbeatInterval):
			routine()
		}
	}
}

func (w *Worker) initTrafficGate() error {
	service := w.service.GetServiceSpec(w.serviceName)

	if err := w.ingressServer.CreateIngress(service, w.applicationPort); err != nil {
		return fmt.Errorf("create ingress for service %s failed: %v", w.serviceName, err)
	}

	if err := w.egressServer.CreateEgress(service); err != nil {
		return fmt.Errorf("create egress for service %s failed: %v", w.serviceName, err)
	}

	return nil
}

func (w *Worker) updateHearbeat() error {
	resp, err := http.Get(w.aliveProbe)
	if err != nil {
		return fmt.Errorf("probe %s check service %s instanceID %s heartbeat failed: %v",
			w.aliveProbe, w.serviceName, w.instanceID, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s check service %s instanceID %s heartbeat failed: status code is %d",
			w.aliveProbe, w.serviceName, w.instanceID, resp.StatusCode)
	}

	logger.Debugf("probe %s check service %s instanceID %s heartbeat successfully: %v",
		w.aliveProbe, w.serviceName, w.instanceID, err)

	value, err := w.store.Get(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID))
	if err != nil {
		return fmt.Errorf("get serivce %s instance %s status failed: %v", w.serviceName, w.instanceID, err)
	}

	status := &spec.ServiceInstanceStatus{
		ServiceName: w.serviceName,
		InstanceID:  w.instanceID,
	}
	if value != nil {
		err := yaml.Unmarshal([]byte(*value), status)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", *value, err)

			// NOTE: This is a little strict, maybe we could use the brand new status to udpate.
			return err
		}
	}

	status.LastHeartbeatTime = time.Now().Format(time.RFC3339)
	buff, err := yaml.Marshal(status)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", status, err)
		return err
	}

	return w.store.Put(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID), string(buff))
}

func (w *Worker) addEgressWatching(serviceName string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			w.egressServer.DeletePipeline(serviceName)
			return false
		case informer.EventUpdate:
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
						w.superSpec.Name(), err, debug.Stack())
				}
			}()
			logger.Infof("handle informer egress service:%s's spec update event", serviceName)
			instanceSpecs := w.service.ListServiceInstanceSpecs(service.Name)

			if err := w.egressServer.UpdatePipeline(service, instanceSpecs); err != nil {
				logger.Errorf("handle informer egress failed, update serivce:%s's failed, err:%v", serviceName, err)
			}
		}
		return true
	}
	if err := w.informer.OnPartOfServiceSpec(serviceName, informer.AllParts, handleSerivceSpec); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress scope watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}

	handleServiceInstances := func(instanceKvs map[string]*spec.ServiceInstanceSpec) bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					w.superSpec.Name(), err, debug.Stack())
			}
		}()
		logger.Infof("handle informer egress service:%s's spec update event", serviceName)
		serviceSpec := w.service.GetServiceSpec(serviceName)

		var instanceSpecs []*spec.ServiceInstanceSpec
		for _, v := range instanceKvs {
			instanceSpecs = append(instanceSpecs, v)
		}
		if err := w.egressServer.UpdatePipeline(serviceSpec, instanceSpecs); err != nil {
			logger.Errorf("handle informer egress failed, update service:%s failed, err:%v", serviceName, err)
		}

		return true
	}

	if err := w.informer.OnServiceInstanceSpecs(serviceName, handleServiceInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress prefix watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}
}

func (w *Worker) informObservability() error {
	handleServiceObservability := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			return false
		case informer.EventUpdate:
			if err := w.observabilityManager.UpdateObservability(w.serviceName, service.Observability); err != nil {
				logger.Errorf("update observability failed: %v", err)
			}
		}

		return true
	}

	err := w.informer.OnPartOfServiceSpec(w.serviceName, informer.ServiceObservability, handleServiceObservability)
	if err != nil && err != informer.ErrAlreadyWatched {
		return fmt.Errorf("on informer for observability failed: %v", err)
	}

	return nil
}

func (w *Worker) watchEvent() {
	for {
		select {
		case <-w.done:
			return
		case name := <-w.egressEvent:
			w.addEgressWatching(name)
		}
	}
}

// Status returns the status of worker.
func (w *Worker) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

// Close close the worker
func (w *Worker) Close() {
	close(w.done)

	w.informer.Close()
	w.registryServer.Close()
}
