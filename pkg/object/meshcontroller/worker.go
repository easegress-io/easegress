package meshcontroller

import (
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
)

// Worker is a sidecar in service mesh
type Worker struct {
	mss *MeshServiceServer

	HeartbeatInterval string
	// InstancePort is the Java Process's listening port
	InstancePort uint32
	// Registried indicated whether the serivce instance registried or not
	Registried bool

	// only one ingress need to watch
	watchIngressPipelineName []string

	// one or moe Egress Pipeline name can be wathed
	watchEgressPipelineName []string

	egress chan map[string]string

	mux   sync.Mutex
	store mockEtcdClient

	done chan struct{}
}

// Run is the entry of the Master role controller
func (w *Worker) Run() {
	watchInterval, err := time.ParseDuration(w.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat duration %s failed: %v",
			w.HeartbeatInterval, err)
		return
	}

	go w.mss.WatchLocalInstaceHearbeat(watchInterval)
	for {
		select {
		case <-w.done:
			return
		}
	}

	return
}

// watchInstanceHeartBeat
func (w *Worker) watchInstanceHeartbeat() {

	// call Java Process with JMX

	// update record into ETCD

	return
}

// Close close the worker
func (w *Worker) Close() {
	w.done <- struct{}{}
}
