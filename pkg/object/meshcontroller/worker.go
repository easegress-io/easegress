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
)

// Worker is a sidecar in service mesh
type Worker struct {
	super     *supervisor.Supervisor
	superSpec *supervisor.Spec
	spec      *Spec

	registried bool

	instanceID string

	rcs  *RegistryCenterServer
	mss  *MeshServiceServer
	ings *IngressServer
	engs *EgressServer

	ingsChan chan IngressMsg
	mux      sync.Mutex

	done chan struct{}
}

// NewWorker returns a initialized worker
func NewWorker(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*Spec)
	ingressNotifyChan := make(chan IngressMsg, defaultIngressChannelBuffer)

	store := &mockEtcdClient{}
	registryCenterServer := NewDefaultRegistryCenterServer(spec.RegistryType, store, ingressNotifyChan)
	serviceServer := NewDefaultMeshServiceServer(store, ingressNotifyChan)
	ingressServer := NewDefualtIngressServer(store, super)

	w := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,

		rcs:      registryCenterServer,
		mss:      serviceServer,
		ings:     ingressServer,
		ingsChan: ingressNotifyChan,

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
		return err
	}

	sidecarSpec, err := w.mss.GetSidecarSepc(w.spec.ServiceName)

	if err != nil {
		return err
	}

	w.rcs.RegistryServiceInstance(ins, serviceSpec, sidecarSpec)

	return err
}

// watchHeartBeat
func (w *Worker) watchHeartbeat(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			if err := w.mss.CheckLocalInstaceHearbeat(w.spec.ServiceName); err != nil {
				logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
			}
		case <-done:
			return
		}
	}

}

func (w *Worker) watchSpecs(interval time.Duration, done chan struct{}) {
	for {
		select {
		case <-time.After(interval):
			if err := w.mss.CheckSpecs(); err != nil {
				logger.Errorf("worker check local instance heartbeat failed, err :%v", err)
			}
		case msg := <-w.ingsChan:
			if err := w.ings.HandleIngressOpMsg(msg); err != nil {
				logger.Errorf("work handle ingress msg failed,msg : %v, err :  %v ", msg, err)
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
