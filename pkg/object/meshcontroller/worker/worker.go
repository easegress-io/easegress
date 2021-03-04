package worker

import (
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registry"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
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

	store := storage.New(superSpec.Name(), super.Cluster())
	registryCenterServer := registry.NewRegistryCenterServer(spec.RegistryType, store)
	ingressServer := NewIngressServer(store, super)

	w := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,
		store:     store,

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

	doneHeartBeat := make(chan struct{})
	doneWatchSpec := make(chan struct{})
	go w.watchHeartbeat(watchInterval, doneHeartBeat)
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

// Registry is a HTTP handler for worker
func (w *Worker) Registry(ctx iris.Context) error {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}
	ins, err := w.rcs.DecodeBody(body)
	if err != nil {
		return err
	}

	serviceYAML, err := w.store.Get(fmt.Sprint(storage.ServiceSpecFormat, ins.ServiceName))
	if err != nil {
		return err
	}

	var service spec.Service
	if err = yaml.Unmarshal([]byte(*serviceYAML), &service); err != nil {
		return err
	}

	if ID := w.rcs.RegistryServiceInstance(ins, &service); len(ID) != 0 {
		w.mux.Lock()
		defer w.mux.Unlock()

		// let worker know its identity
		w.instanceID = ID
		w.serviceName = ins.ServiceName
		// asynchronous add watch ingress spec keys
		go w.addWatchIngressSpecNames(ins.ServiceName)
	}

	return err
}

// watchHeartBeat
func (w *Worker) watchHeartbeat(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			// once its instanceID and serivceName be setted,
			if w.rcs.Registried {
				if err := w.CheckLocalInstaceHeartbeat(); err != nil {
					logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
				}
			}
		case <-done:
			return
		}
	}

}

// CheckLocalInstaceHeartbeat communicate with Java process and check its health.
func (w *Worker) CheckLocalInstaceHeartbeat() error {
	var (
		alive bool
	)

	//[TODO] call Java process agent with JMX, check it alive

	if alive == true {
		//storage.ServiceInstanceHeartbeatFormat, w.serivceName, w.instanceID

		/*heartbeat.LastActiveTime = time.Now().Unix()

		return mss.setServiceInstanceHeartbeat(serviceName, ID, heartbeat)
		*/

	} else {
		// do nothing, master will notice this irregular
		// and cause the update of Egress's Pipelines which are relied
		// on this instance
	}

	return nil
}

// addWatchIngressSpecsNames calls meshServiceServer to add Ingress's
// HTTPServer and Pipeline spec name into watch list
func (w *Worker) addWatchIngressSpecNames(serviceName string) {
	/*for {
		if err := w.mss.addWatchIngressSpecNames(serviceName); err == nil {
			break
		} else {
			// retry add watch spec names
			logger.Errorf("worker add service :%s, ingress watch spec names failed, err :%v", serviceName, err)
			time.Sleep(defaultWatchRetryTimeSecond * time.Second)
		}
	}*/
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
func (w *Worker) getServiceInstanceHeartbeat(serviceName, ID string) (*spec.Heartbeat, error) {
	var (
		err           error
		heartbeatYAML *string
		heartbeat     spec.Heartbeat
	)
	key := fmt.Sprintf(storage.ServiceInstanceHeartbeatFormat, serviceName, ID)
	if heartbeatYAML, err = w.store.Get(key); err != nil {
		if heartbeatYAML == nil {
			//
		} else {
			logger.Errorf("get service : %s's heartbeat %s from store failed, err :%v", serviceName, key, err)
		}
		return &heartbeat, err
	} else {
		if err = yaml.Unmarshal([]byte(*heartbeatYAML), &heartbeat); err != nil {
			logger.Errorf("BUG, serivce : %s ummarshal yaml failed, spec :%s, err : %v", serviceName, heartbeatYAML, err)
			return &heartbeat, err
		}
	}

	return &heartbeat, err
}
