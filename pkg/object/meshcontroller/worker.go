package meshcontroller

import (
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/logger"
)

// Worker is a sidecar in service mesh
type Worker struct {
	ServiceName       string
	HeartbeatInterval string
	// InstancePort is the Java Process's listening port
	InstancePort uint32
	// Registried indicated whether the serivce instance registried or not
	Registried bool

	rcs  *RegistryCenterServer
	mss  *MeshServiceServer
	ings *IngressServer
	engs *EgressServer

	mux sync.Mutex

	done chan struct{}
}

// NewWorker news a mesh worker by spec
func NewWorker(spec *Spec) *Worker {

	store := &mockEtcdClient{}
	registryCenterServer := &RegistryCenterServer{
		RegistryType: spec.RegistryType,
		store:        store,
	}

	serviceServer := &MeshServiceServer{
		store: store,
	}

	ingressServer := &IngressServer{
		store: store,
	}
	w := &Worker{
		rcs:  registryCenterServer,
		mss:  serviceServer,
		ings: ingressServer,
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

	// watch the local instance heartbeat
	go w.watchInstanceHeartbeat(watchInterval)

	// watch the corrspoding instance record, if delete or updated, should modify the ingress
	go w.watchServiceInstancesRecord(watchInterval)
	for {
		select {
		case <-w.done:
			return
		}
	}
}

//
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

	w.rcs.RegistryServiceInstance(ins, serviceSpec, sidecarSpec, w.ings)

	return err
}

// watchInstanceHeartBeat
func (w *Worker) watchInstanceHeartbeat(interval time.Duration) {

	w.mss.WatchLocalInstaceHearbeat(interval)

	return
}

func (w *Worker) watchServiceInstancesRecord(interval time.Duration) {
	// get all service instances

}

// Close close the worker
func (w *Worker) Close() {
	w.done <- struct{}{}
}
