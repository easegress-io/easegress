package worker

import (
	"net/http"
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
	// handle worker inner logic
	instanceID          string
	serviceName         string
	aliveProbe          string
	store               storage.Storage
	rcs                 *registrycenter.Server
	ings                *IngressServer
	inf                 informer.Informer
	egs                 *EgressServer
	observabilityServer *ObservabilityManager
	mutex               sync.Mutex

	egressWatch chan string
	done        chan struct{}
}

var defaultWathChanBuffer = 100

// New creates a mesh worker.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)
	serviceName := option.Global.Labels["mesh-servicename"]
	aliveProbe := option.Global.Labels["alive-probe"]
	store := storage.New(superSpec.Name(), super.Cluster())
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, store)
	ingressServer := NewIngressServer(super, serviceName)
	egressWatch := make(chan string, defaultWathChanBuffer)
	egressServer := NewEgressServer(super, serviceName, store, egressWatch)
	observabilityServer := NewObservabilityServer(serviceName)

	inf := informer.NewInformer(store)

	w := &Worker{
		super:       super,
		superSpec:   superSpec,
		spec:        spec,
		store:       store,
		serviceName: serviceName,
		aliveProbe:  aliveProbe,

		rcs:                 registryCenterServer,
		ings:                ingressServer,
		egs:                 egressServer,
		observabilityServer: observabilityServer,
		inf:                 inf,

		egressWatch: egressWatch,
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
		logger.Errorf("mesh service name is empty!")
		return
	}

	if len(w.aliveProbe) == 0 || strings.HasPrefix(w.aliveProbe, "http://") {
		logger.Errorf("mesh worker alive check probe :[%s] is invalide!", w.aliveProbe)
		return
	}

	doneHeartBeat := make(chan struct{})
	doneWatchSpec := make(chan struct{})
	go w.heartbeat(watchInterval, doneHeartBeat)
	go w.watchEvents(doneWatchSpec)

	<-w.done
	close(doneHeartBeat)
	close(doneWatchSpec)
	w.inf.Close()
}

// heartbeat checks local instance's java process's aliveness and
// update its heartbeat recored.
func (w *Worker) heartbeat(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			// only check after worker registry itself successfully
			if w.rcs.Registried() {
				if err := w.checkLocalInstanceHeartbeat(); err != nil {
					logger.Errorf("worker checks local instance heartbeat failed: %v", err)
				}
				// add watch self's spec for ingress
				w.addWatchIngress()
			}
		case <-done:
			return
		}
	}

}

// checkLocalInstanceHeartbeat using alive-probe URL to check
// local Java process's alive, then update local instance's hearbeat.
func (w *Worker) checkLocalInstanceHeartbeat() error {
	var alive bool
	resp, err := http.Get(w.aliveProbe)
	if err != nil {
		logger.Errorf("worker checks service %s, instanceID :%s, heartbeat failed, probe url :%s, err :%v",
			w.serviceName, w.instanceID, w.aliveProbe, err)
		alive = false
	} else {
		if resp.StatusCode == http.StatusOK {
			logger.Infof("worker checks heartbeat succ, serviceName:%s, instancdID:%s, probeURL:%s",
				w.serviceName, w.instanceID, w.aliveProbe)
			alive = true
		} else {
			alive = false
			logger.Errorf("worker checks heartbeat without HTTP 200, serviceName:%s, instancdID:%s, probeURL:%s, statuscode:%d",
				w.serviceName, w.instanceID, w.aliveProbe, resp.StatusCode)
		}
	}

	if alive {
		value, err := w.store.Get(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID))
		if err != nil {
			logger.Errorf("worker gets serivce %s/%s status failed: %v", w.serviceName, w.instanceID, err)
			return err
		}

		status := &spec.ServiceInstanceStatus{}
		if value != nil {
			if err := yaml.Unmarshal([]byte(*value), status); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %s", *value, err)
				return err
			}
		}
		var buff []byte
		status.LastHeartbeatTime = time.Now().Format(time.RFC3339)
		if buff, err = yaml.Marshal(status); err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v",
				w.serviceName, status, err)
			return err
		}

		return w.store.Put(layout.ServiceInstanceStatusKey(w.serviceName, w.instanceID), string(buff))
	}

	// do nothing, master will notice this irregular
	// and cause the update of Egress's Pipelines which are relied
	// on this instance

	return nil
}

// getSerivceInstances get whole service Instances from store.
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
		logger.Errorf("worker gets %s failed: %v", serviceName, err)
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

// addEgressWatch adds one egress service spec and instance list wathcing by using
// informer
func (w *Worker) addEgressWatch(serviceName string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			w.egs.DeletePipeline(serviceName)
			return false
		case informer.EventUpdate:
			logger.Infof("worker handle egress service:%s's spec update event", serviceName)

			ins, err := getSerivceInstances(service.Name, w.store)
			if err != nil {
				logger.Errorf("egresss add watching by informer failed, get service:[%s] instance list failed, err:%v", serviceName, err)
				return true
			}
			if err := w.egs.UpdatePipeline(service, ins); err != nil {
				logger.Errorf("egress update pipeline by informer failed, update serivce:%s's failed, err:%v", serviceName, err)
			}
		}
		return true
	}
	if err := w.inf.OnPartOfServiceSpec(serviceName, informer.AllParts, handleSerivceSpec); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("worker add egress scope watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}

	handleServiceInstances := func(insMap map[string]*spec.ServiceInstanceSpec) bool {
		logger.Infof("worker handle egress service:%s's spec update event", serviceName)
		service, err := getService(serviceName, w.store)
		if err != nil {
			logger.Errorf("worker add egress watching by informer failed, get service:%s spec failed, err:%v", serviceName, err)
			return true
		}
		var insList []*spec.ServiceInstanceSpec
		for _, v := range insMap {
			insList = append(insList, v)
		}
		if err := w.egs.UpdatePipeline(service, insList); err != nil {
			logger.Errorf("worker update pipeline by informer failed,update service:%s failed, err:%v", serviceName, err)
		}

		return true
	}

	if err := w.inf.OnServiceInstanceSpecs(serviceName, handleServiceInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("worker add egress prefix watching failed, service:%s, err:%v", serviceName, err)
			return
		}
	}
}

// addWatchIngress will watch ingress spec after worker registried itself successfully.
func (w *Worker) addWatchIngress() {
	// add watch ingress
	handleServiceObservability := func(event informer.Event, service *spec.Service) bool {
		switch event {
		case informer.EventDelete:
			logger.Infof("worker handle ingress service:%s's spec delete event", w.serviceName)
			return false
		case informer.EventUpdate:
			logger.Infof("worker handle ingress service:%s's spec update event", w.serviceName)

			if err := w.observabilityServer.UpdateObservability(w.serviceName, service.Observability); err != nil {
				logger.Errorf("worker call observability server to notify Java process failed, observability spec:%#v,err:%v ", service.Observability, err)
			}
		}
		return true
	}

	if err := w.inf.OnPartOfServiceSpec(w.serviceName, informer.ServiceObservability, handleServiceObservability); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("worker add ingress socpe:%s faile, err:%v", informer.ServiceObservability, err)
		}
	}
}

// watchEvents checks worker's using
// egress service specs's udpate/delete, instance list udpated
// by calling Informer, then apply modification into egress server
func (w *Worker) watchEvents(done chan struct{}) {
	// watch egress watch chain continuously, because it will dynamically changed
	// according to Java processes RPC behaviour
	for {
		select {
		case name := <-w.egressWatch:
			w.addEgressWatch(name)
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
