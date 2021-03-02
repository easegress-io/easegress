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
	// ServiceName indicates which service this work servers to
	ServiceName string
	// HeartbeatInterval is the interval for loal Java process's alive heartbeat
	HeartbeatInterval string
	// SpecUpdateInterval  is
	SpecUpdateInterval string
	// Registried indicated whether the serivce instance registried or not
	Registried bool

	// InstanceID is this work
	InstanceID string

	rcs  *RegistryCenterServer
	mss  *MeshServiceServer
	ings *IngressServer
	engs *EgressServer

	ingsChan chan IngressMsg
	mux      sync.Mutex

	done chan struct{}
}

// NewWorker returns a initialized worker
func NewWorker(spec *Spec, super *supervisor.Supervisor) *Worker {

	ingressNotifyChan := make(chan IngressMsg, defaultIngressChannelBuffer)

	store := &mockEtcdClient{}
	registryCenterServer := NewDefaultRegistryCenterServer(spec.RegistryType, store, ingressNotifyChan)
	serviceServer := NewDefaultMeshServiceServer(store, ingressNotifyChan)
	ingressServer := NewDefualtIngressServer(store, super)

	w := &Worker{
		ServiceName:       spec.ServiceName,
		HeartbeatInterval: spec.HeartbeatInterval,

		rcs:      registryCenterServer,
		mss:      serviceServer,
		ings:     ingressServer,
		ingsChan: ingressNotifyChan,
	}
	return w
}

// Run is the entry of the Master role controller
func (w *Worker) Run() {
	watchInterval, err := time.ParseDuration(w.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat duration %s failed: %v",
			w.HeartbeatInterval, err)
		return
	}

	specUpdateInterval, err := time.ParseDuration(w.SpecUpdateInterval)
	if err != nil {
		logger.Errorf("BUG: parse spec update duration %s failed: %v",
			w.SpecUpdateInterval, err)
		return
	}

	var doneHeartBeat chan struct{}
	var doneWatchSpec chan struct{}

	go w.watchHeartbeat(watchInterval, doneHeartBeat)
	go w.watchSpecs(specUpdateInterval, doneWatchSpec)

	for {
		select {

		case <-w.done:
			doneHeartBeat <- struct{}{}
			doneWatchSpec <- struct{}{}
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

	w.ings.SetIngressPipelinePort(ins.Port)

	serviceSpec, err := w.mss.GetServiceSpec(w.ServiceName)

	if err != nil {
		return err
	}

	sidecarSpec, err := w.mss.GetSidecarSepc(w.ServiceName)

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
			if err := w.mss.CheckLocalInstaceHearbeat(w.ServiceName); err != nil {
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

// Close close the worker
func (w *Worker) Close() {
	w.done <- struct{}{}
}
