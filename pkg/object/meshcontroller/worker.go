package meshcontroller

import (
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/logger"
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
	spec      *Spec

	// handle worker inner logic
	instanceID string
	rcs        *RegistryCenterServer
	mss        *MeshServiceServer
	ings       *IngressServer
	engs       *EgressServer
	mux        sync.Mutex

	ingsChan chan IngressMsg
	engsChan chan EngressMsg
	done     chan struct{}
}

// NewWorker returns a initialized worker
func NewWorker(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*Spec)
	ingressNotifyChan := make(chan IngressMsg, defaultIngressChannelBuffer)
	engressNotifyChan := make(chan EngressMsg, defaultEngressChannelBuffer)

	store := &mockEtcdClient{}
	registryCenterServer := NewRegistryCenterServer(spec.RegistryType, store, ingressNotifyChan)
	serviceServer := NewMeshServiceServer(store, spec.AliveSeconds, ingressNotifyChan)
	ingressServer := NewIngressServer(store)

	w := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,

		rcs:      registryCenterServer,
		mss:      serviceServer,
		ings:     ingressServer,
		ingsChan: ingressNotifyChan,
		engsChan: engressNotifyChan,

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

	specUpdateInterval, err := time.ParseDuration(w.spec.SpecUpdateInterval)
	if err != nil {
		logger.Errorf("BUG: parse spec update duration %s failed: %v",
			w.spec.SpecUpdateInterval, err)
		return
	}

	doneHeartBeat := make(chan struct{})
	doneWatchSpec := make(chan struct{})
	go w.watchHeartbeat(watchInterval, doneHeartBeat)
	go w.watchSpecs(specUpdateInterval, doneWatchSpec)

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
	ins, err := w.rcs.decodeBody(body)
	if err != nil {
		return err
	}

	serviceSpec, err := w.mss.GetServiceSpec(w.spec.ServiceName)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}

	sidecarSpec, err := w.mss.GetSidecarSepc(w.spec.ServiceName)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}

	if ID := w.rcs.RegistryServiceInstance(ins, serviceSpec, sidecarSpec); len(ID) != 0 {
		w.mux.Lock()
		defer w.mux.Unlock()

		w.instanceID = ID
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
			if err := w.mss.CheckLocalInstaceHeartbeat(w.spec.ServiceName, w.instanceID); err != nil {
				logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
			}
		case <-done:
			return
		}
	}

}

// addWatchIngressSpecsNames calls meshServiceServer to add Ingress's
// HTTPServer and Pipeline spec name into watch list
func (w *Worker) addWatchIngressSpecNames(serviceName string) {
	for {
		if err := w.mss.addWatchIngressSpecNames(serviceName); err == nil {
			break
		} else {
			// retry add watch spec names
			logger.Errorf("worker add service :%s, ingress watch spec names failed, err :%v", serviceName, err)
			time.Sleep(defaultWatchRetryTimeSecond * time.Second)
		}
	}
}

// watchSpecs calls meshServiceServer check specs udpate/create/delete opertion
// and apply this modification into memory
func (w *Worker) watchSpecs(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			if err := w.mss.CheckSpecs(); err != nil {
				logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
			}
		case msg := <-w.ingsChan:
			if err := w.ings.HandleIngressOpMsg(msg); err != nil {
				logger.Errorf("worker handle ingress msg failed,msg : %v, err :  %v ", msg, err)
			}
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
