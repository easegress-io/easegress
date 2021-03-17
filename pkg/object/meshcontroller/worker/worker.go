package worker

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/supervisor"
)

// Worker is a sidecar in service mesh
type Worker struct {
	super     *supervisor.Supervisor
	superSpec *supervisor.Spec
	spec      *spec.Admin

	// set by labels
	instanceID      string
	serviceName     string
	aliveProbe      string
	applicationPort uint32

	store               storage.Storage
	rcs                 *registrycenter.Server
	ings                *IngressServer
	inf                 informer.Informer
	egs                 *EgressServer
	observabilityServer *ObservabilityManager
	mutex               sync.Mutex

	egressEvent chan string
	done        chan struct{}
}

const (
	labelsApplictionPort = "application-port"
	labelsAliveProbe     = "alive-probe"
	labelsServiceName    = "mesh-servicename"
)

var defaultWathChanBuffer = 100

// New creates a mesh worker.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)

	serviceName := option.Global.Labels[labelsServiceName]
	aliveProbe := option.Global.Labels[labelsAliveProbe]
	applicationPort, _ := strconv.Atoi(option.Global.Labels[labelsApplictionPort])

	store := storage.New(superSpec.Name(), super.Cluster())
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, store)
	ingressServer := NewIngressServer(super, serviceName)
	egressEvent := make(chan string, defaultWathChanBuffer)
	egressServer := NewEgressServer(super, serviceName, store, egressEvent)
	observabilityServer := NewObservabilityServer(serviceName)
	inf := informer.NewInformer(store)

	w := &Worker{
		super:           super,
		superSpec:       superSpec,
		spec:            spec,
		store:           store,
		serviceName:     serviceName,
		aliveProbe:      aliveProbe,
		applicationPort: uint32(applicationPort),

		rcs:                 registryCenterServer,
		ings:                ingressServer,
		egs:                 egressServer,
		observabilityServer: observabilityServer,
		inf:                 inf,

		egressEvent: egressEvent,
		done:        make(chan struct{}),
	}

	w.registerAPIs()

	go w.run()

	return w
}

func (w *Worker) run() {
	watchInterval, err := time.ParseDuration(w.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat duration %s failed: %v",
			w.spec.HeartbeatInterval, err)
		return
	}

	if len(w.serviceName) == 0 {
		logger.Errorf("check mesh service name failed, empty!")
		return
	}

	if len(w.aliveProbe) == 0 || strings.HasPrefix(w.aliveProbe, "http://") {
		logger.Errorf("check alive probe:[%s] failed, invalide!", w.aliveProbe)
		return
	}

	if w.applicationPort == 0 {
		logger.Errorf("check java process port:[%d] failed, value in start labels:[%s], invalide!",
			w.applicationPort, option.Global.Labels[labelsApplictionPort])
		return
	}

	doneHeartBeat := make(chan struct{})
	doneWatchEvent := make(chan struct{})
	go w.heartbeat(watchInterval, doneHeartBeat)
	go w.watchEvent(doneWatchEvent)

	<-w.done
	close(doneHeartBeat)
	close(doneWatchEvent)
	w.inf.Close()
	w.rcs.Close()
	w.ings.Close()
	w.egs.Close()
}

func (w *Worker) heartbeat(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			w.initTrafficGate()

			if w.rcs.Registried() {
				if err := w.updateHearbeat(); err != nil {
					logger.Errorf("check local instance heartbeat failed:%v", err)
				}
				w.addIngressWatching()
			}
		case <-done:
			return
		}
	}
}

func (w *Worker) initTrafficGate() {
	service, err := getService(w.serviceName, w.store)
	if err != nil {
		logger.Errorf("get worker's service:%s failed,err:%v", w.serviceName, err)
		return
	}

	if err := w.ings.CreateIngress(service, w.applicationPort); err != nil {
		logger.Errorf("create ingress failed: %v", w.serviceName, err)
		return
	}

	if err := w.egs.CreateEgress(service); err != nil {
		logger.Errorf("create egress failed: %v", w.serviceName, err)
		return
	}
}

func (w *Worker) updateHearbeat() error {
	var alive bool
	resp, err := http.Get(w.aliveProbe)
	if err != nil {
		logger.Errorf("check service:%s, instanceID:%s, heartbeat failed, probe url:%s, err:%v",
			w.serviceName, w.instanceID, w.aliveProbe, err)
		alive = false
	} else {
		if resp.StatusCode == http.StatusOK {
			logger.Infof("check heartbeat succ, serviceName:%s, instancdID:%s, probeURL:%s",
				w.serviceName, w.instanceID, w.aliveProbe)
			alive = true
		} else {
			alive = false
			logger.Errorf("check heartbeat without HTTP 200, serviceName:%s, instancdID:%s, probeURL:%s, statuscode:%d",
				w.serviceName, w.instanceID, w.aliveProbe, resp.StatusCode)
		}
	}

	if alive {
		value, err := w.store.Get(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID))
		if err != nil {
			logger.Errorf("get serivce:%s, instance:%s status failed:%v", w.serviceName, w.instanceID, err)
			return err
		}

		status := &spec.ServiceInstanceStatus{}
		if value != nil {
			if err := yaml.Unmarshal([]byte(*value), status); err != nil {
				logger.Errorf("BUG: unmarshal val:%s to yaml failed:%v", *value, err)
				return err
			}
		}

		var buff []byte
		status.LastHeartbeatTime = time.Now().Format(time.RFC3339)
		if buff, err = yaml.Marshal(status); err != nil {
			logger.Errorf("BUG: marshal service:%s, val:%#v to yaml failed: %v",
				w.serviceName, status, err)
			return err
		}
		return w.store.Put(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID), string(buff))
	}

	return nil
}

func getSerivceInstances(serviceName string, store storage.Storage) ([]*spec.ServiceInstanceSpec, error) {
	var insList []*spec.ServiceInstanceSpec

	insYAMLs, err := store.GetPrefix(layout.ServiceInstanceSpecPrefix(serviceName))
	if err != nil {
		return insList, err
	}

	for _, v := range insYAMLs {
		var ins *spec.ServiceInstanceSpec
		if err = yaml.Unmarshal([]byte(v), ins); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		insList = append(insList, ins)
	}

	return insList, nil
}

func getService(serviceName string, store storage.Storage) (*spec.Service, error) {
	var (
		service *spec.Service
		err     error
	)
	serviceSpec, err := store.Get(layout.ServiceSpecKey(serviceName))
	if err != nil {
		logger.Errorf("get %s failed: %v", serviceName, err)
		return nil, err
	}

	if len(*serviceSpec) == 0 {
		return nil, spec.ErrServiceNotFound
	}

	err = yaml.Unmarshal([]byte(*serviceSpec), service)
	if err != nil {
		logger.Errorf("BUG: unmarshal %s to yaml failed: %v", serviceName, err)
		return nil, err
	}
	return service, nil
}

func (w *Worker) addEgressWatching(serviceName string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			w.egs.DeletePipeline(serviceName)
			return false
		case informer.EventUpdate:
			logger.Infof("handle informer egress service:%s's spec update event", serviceName)
			ins, err := getSerivceInstances(service.Name, w.store)
			if err != nil {
				logger.Errorf("handle informer egress failed, get service:[%s] instance list failed, err:%v", serviceName, err)
				return true
			}
			if err := w.egs.UpdatePipeline(service, ins); err != nil {
				logger.Errorf("handle informer egress failed, update serivce:%s's failed, err:%v", serviceName, err)
			}
		}
		return true
	}
	if err := w.inf.OnPartOfServiceSpec(serviceName, informer.AllParts, handleSerivceSpec); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress scope watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}

	handleServiceInstances := func(insMap map[string]*spec.ServiceInstanceSpec) bool {
		logger.Infof("handle informer egress service:%s's spec update event", serviceName)
		service, err := getService(serviceName, w.store)
		if err != nil {
			logger.Errorf("hanlde informer egress failed, get service:%s spec failed, err:%v", serviceName, err)
			return true
		}
		var insList []*spec.ServiceInstanceSpec
		for _, v := range insMap {
			insList = append(insList, v)
		}
		if err := w.egs.UpdatePipeline(service, insList); err != nil {
			logger.Errorf("handle informer egress failed, update service:%s failed, err:%v", serviceName, err)
		}

		return true
	}

	if err := w.inf.OnServiceInstanceSpecs(serviceName, handleServiceInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress prefix watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}
}

func (w *Worker) addIngressWatching() {
	handleServiceObservability := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			logger.Infof("handle informer ingress service:%s's spec delete event", w.serviceName)
			return false
		case informer.EventUpdate:
			logger.Infof("handle informer ingress service:%s's spec update event", w.serviceName)

			if err := w.observabilityServer.UpdateObservability(w.serviceName, service.Observability); err != nil {
				logger.Errorf("hanlde informer ingress, call observability server to notify Java process failed, observability spec:%#v,err:%v ",
					service.Observability, err)
			}
		}
		return true
	}

	if err := w.inf.OnPartOfServiceSpec(w.serviceName, informer.ServiceObservability, handleServiceObservability); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add ingress scope:%s faile, err:%v", informer.ServiceObservability, err)
		}
	}
}

func (w *Worker) watchEvent(done chan struct{}) {
	for {
		select {
		case name := <-w.egressEvent:
			w.addEgressWatching(name)
		case <-done:
			return
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
}
