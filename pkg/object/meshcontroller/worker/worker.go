package worker

import (
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registry"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/supervisor"
)

const (
	defaultIngressChannelBuffer = 100
	defaultEngressChannelBuffer = 200
	defaultWatchRetryTimeSecond = 2
)

// Worker is a sidecar in service mesh
type Worker struct {
	super     *supervisor.Supervisor
	superSpec *supervisor.Spec
	spec      *spec.Admin
	// handle worker inner logic
	instanceID  string
	serviceName string
	store       storage.Storage
	rcs         *registry.RegistryCenterServer
	ings        *IngressServer
	engs        *EgressServer
	mux         sync.Mutex

	done chan struct{}
}

// New creates a mesh worker.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)
	serviceName := option.Global.Labels["mesh_servicename"]
	store := storage.New(superSpec.Name(), super.Cluster())
	registryCenterServer := registry.NewRegistryCenterServer(spec.RegistryType, serviceName, store)
	ingressServer := NewIngressServer(super)

	w := &Worker{
		super:       super,
		superSpec:   superSpec,
		spec:        spec,
		store:       store,
		serviceName: serviceName,

		rcs:  registryCenterServer,
		ings: ingressServer,

		done: make(chan struct{}),
	}

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
	go w.Heartbeat(watchInterval, doneHeartBeat)
	go w.watchSpecs(doneWatchSpec)

	for {
		select {

		case <-w.done:
			close(doneHeartBeat)
			close(doneWatchSpec)
			return
		}
	}
}

// Heartbeat check local instance's java process's aliveness and
// update its heartbeat recored
func (w *Worker) Heartbeat(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			// once it registried itself successfully,
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

// checkLocalInstanceHeartbeat communicate with Java process and check its health.
func (w *Worker) checkLocalInstanceHeartbeat() error {
	var alive bool

	//[TODO] call Java process agent with JMX, check it alive
	if alive == true {
		heartBeatYAML, err := w.store.Get(layout.GenServiceHeartbeatKey(w.serviceName, w.instanceID))
		if err != nil {
			logger.Errorf("get serivce %s, instace :%s , heartbeat failed, err : %v", w.serviceName, w.instanceID, err)
		}

		var heartbeat spec.Heartbeat
		if heartBeatYAML != nil {
			if err := yaml.Unmarshal([]byte(*heartBeatYAML), &heartbeat); err != nil {
				return err
			}
		}
		var buff []byte
		heartbeat.LastActiveTime = time.Now().Unix()
		if buff, err = yaml.Marshal(&heartbeat); err != nil {
			return err
		}

		err = w.store.Put(layout.GenServiceHeartbeatKey(w.serviceName, w.instanceID), string(buff))
		return err
	} else {
		// do nothing, master will notice this irregular
		// and cause the update of Egress's Pipelines which are relied
		// on this instance
	}

	return nil
}

// watchSpecs calls meshServiceServer check specs udpate/create/delete opertion
// and apply this modification into memory
func (w *Worker) watchSpecs(done chan struct{}) {
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
