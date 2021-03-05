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

		mss                  *MeshServiceServer
		serviceWatchInterval string

		done chan struct{}
	}
)

// New creates a mesh master.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Master {
	storage := storage.New(superSpec.Name(), super.Cluster())
	adminSpec := superSpec.ObjectSpec().(*spec.Admin)

	heartbeatInterval, err := time.ParseDuration(adminSpec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse %s to duration failed: %v", adminSpec.HeartbeatInterval, err)
	}

	// 2 times of heartbeatInterval for judging whether a service is
	// alive or not
	aliveSeconds := 2 * int64(heartbeatInterval.Seconds())
	serviceServer := NewMeshServiceServer(storage, aliveSeconds)

	m := &Master{
		super:     super,
		superSpec: superSpec,
		spec:      adminSpec,

		mss:  serviceServer,
		done: make(chan struct{}),
	}

	go m.run()

	return m
}

func (m *Master) watchServicesHeartbeat() {
	m.mss.WatchSerivceInstancesHeartbeat()

	return
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
