package worker

import (
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
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
	store               storage.Storage
	rcs                 *registrycenter.Server
	ings                *IngressServer
	engs                *EgressServer
	observabilityServer *ObservabilityManager
	mux                 sync.Mutex

	done chan struct{}
}

// New creates a mesh worker.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)
	serviceName := option.Global.Labels["mesh_servicename"]
	store := storage.New(superSpec.Name(), super.Cluster())
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, store)
	ingressServer := NewIngressServer(super, serviceName)
	observabilityServer := NewObservabilityServer(serviceName)

	w := &Worker{
		super:       super,
		superSpec:   superSpec,
		spec:        spec,
		store:       store,
		serviceName: serviceName,

		rcs:                 registryCenterServer,
		ings:                ingressServer,
		observabilityServer: observabilityServer,

		done: make(chan struct{}),
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
		logger.Errorf("mesh servie name is empty!")
		return
	}

	doneHeartBeat := make(chan struct{})
	doneWatchSpec := make(chan struct{})
	go w.heartbeat(watchInterval, doneHeartBeat)
	go w.watchEvents(doneWatchSpec)

	for {
		select {

		case <-w.done:
			close(doneHeartBeat)
			close(doneWatchSpec)
			return
		}
	}
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
					logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
				}
			}
		case <-done:
			return
		}
	}

}

// checkLocalInstanceHeartbeat communicates with Java process locally and check its health.
func (w *Worker) checkLocalInstanceHeartbeat() error {
	var alive bool

	//[TODO] call Java process agent with JMX, check it alive
	if alive == true {
		heartBeatYAML, err := w.store.Get(layout.ServiceHeartbeatKey(w.serviceName, w.instanceID))
		if err != nil {
			logger.Errorf("get serivce %s, instace :%s , heartbeat failed, err : %v",
				w.serviceName, w.instanceID, err)
			return err
		}

		var heartbeat spec.Heartbeat
		if heartBeatYAML != nil {
			if err := yaml.Unmarshal([]byte(*heartBeatYAML), &heartbeat); err != nil {
				logger.Errorf("BUG: unmarsh service :%s, heartbeat :%s, failed, err %s",
					w.serviceName, *heartBeatYAML, err)
				return err
			}
		}
		var buff []byte
		heartbeat.LastActiveTime = time.Now().Unix()
		if buff, err = yaml.Marshal(&heartbeat); err != nil {
			logger.Errorf("BUG: marsh service :%s, heartbeat :%v, failed, err %s",
				w.serviceName, heartbeat, err)
			return err
		}

		return w.store.Put(layout.ServiceHeartbeatKey(w.serviceName, w.instanceID), string(buff))
	}

	// do nothing, master will notice this irregular
	// and cause the update of Egress's Pipelines which are relied
	// on this instance

	return nil
}

// watchEvents checks worker's using
//   1. ingress/egress specs's udpate/delete
//   2. service instance record operation
// by calling Informer, then apply modification into ingress/egerss server
func (w *Worker) watchEvents(done chan struct{}) {
	for {
		select {
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
