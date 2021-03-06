package master

import (
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/supervisor"
)

type (
	// Master is the master role of EaseGateway for mesh control plane.
	Master struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin

		store   storage.Storage
		service *masterService

		done chan struct{}
	}
)

// New creates a mesh master.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Master {
	store := storage.New(superSpec.Name(), super.Cluster())
	adminSpec := superSpec.ObjectSpec().(*spec.Admin)

	m := &Master{
		super:     super,
		superSpec: superSpec,
		spec:      adminSpec,

		store:   store,
		service: newMasterService(superSpec, store),
	}

	m.registerAPIs()

	go m.run()

	return m
}

func (m *Master) watchServicesHeartbeat() {
	m.service.WatchSerivceInstancesHeartbeat()
}

func (m *Master) run() {
	watchInterval, err := time.ParseDuration(m.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			m.spec.HeartbeatInterval, err)
		return
	}

	for {
		select {
		case <-m.done:
			return
		case <-time.After(watchInterval):
			m.watchServicesHeartbeat()
		}
	}
}

// Close closes the master
func (m *Master) Close() {
	close(m.done)
}

// Status returns the status of master.
func (m *Master) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}
