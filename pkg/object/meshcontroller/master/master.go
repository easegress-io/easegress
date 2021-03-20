package master

import (
	"runtime/debug"
	"time"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/service"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/supervisor"
	"gopkg.in/yaml.v2"
)

type (
	// Master is the master role of EaseGateway for mesh control plane.
	Master struct {
		super               *supervisor.Supervisor
		superSpec           *supervisor.Spec
		spec                *spec.Admin
		maxHeartbeatTimeout time.Duration

		store   storage.Storage
		service *service.Service

		done chan struct{}
	}

	// Status is the status of mesh master.
	Status struct {
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
		service: service.New(superSpec, store),

		done: make(chan struct{}),
	}

	heartbeat, err := time.ParseDuration(m.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval %s to duration failed: %v",
			m.spec.HeartbeatInterval, err)
	}
	m.maxHeartbeatTimeout = heartbeat * 2

	m.registerAPIs()

	go m.run()

	return m
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
			func() {
				defer func() {
					if err := recover(); err != nil {
						logger.Errorf("failed to check instance heartbeat %v, stack trace: \n%s\n",
							err, debug.Stack())
					}

				}()
				m.checkInstancesHeartbeat()
			}()
		}
	}
}

func (m *Master) checkInstancesHeartbeat() {
	statuses := m.service.ListAllServiceInstanceStatuses()
	specs := m.service.ListAllServiceInstanceSpecs()

	failedInstances := []*spec.ServiceInstanceSpec{}
	rebornInstances := []*spec.ServiceInstanceSpec{}
	now := time.Now()
	for _, _spec := range specs {
		var status *spec.ServiceInstanceStatus
		for _, s := range statuses {
			if s.ServiceName == _spec.ServiceName && s.InstanceID == _spec.InstanceID {
				status = s
			}
		}
		if status != nil {
			lastHeartbeatTime, err := time.Parse(time.RFC3339, status.LastHeartbeatTime)
			if err != nil {
				logger.Errorf("BUG: parse last heartbeat time %s failed: %v", status.LastHeartbeatTime, err)
				continue
			}
			gap := now.Sub(lastHeartbeatTime)
			if gap > m.maxHeartbeatTimeout {
				logger.Errorf("%s/%s expired for %s", _spec.ServiceName, _spec.InstanceID, gap.String())
				failedInstances = append(failedInstances, _spec)
			} else {
				if _spec.Status == spec.SerivceStatusOutOfSerivce {
					logger.Infof("%s/%s heartbeat recoverd, make it UP", _spec.ServiceName, _spec.InstanceID)
					rebornInstances = append(rebornInstances, _spec)
				}
			}
		} else {
			logger.Errorf("status of %s/%s not found", _spec.ServiceName, _spec.InstanceID)
			failedInstances = append(failedInstances, _spec)
		}
	}

	m.handleFailedInstances(failedInstances)
	m.handleRebornInstances(rebornInstances)
}

func (m *Master) handleRebornInstances(rebornInstances []*spec.ServiceInstanceSpec) {
	m.updateInstanceStatus(rebornInstances, spec.SerivceStatusUp)
}

func (m *Master) handleFailedInstances(failedInstances []*spec.ServiceInstanceSpec) {
	m.updateInstanceStatus(failedInstances, spec.SerivceStatusOutOfSerivce)
}

func (m *Master) updateInstanceStatus(instances []*spec.ServiceInstanceSpec, status string) {
	for _, _spec := range instances {
		_spec.Status = status

		buff, err := yaml.Marshal(_spec)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v", _spec, err)
			continue
		}

		key := layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID)
		err = m.store.Put(key, string(buff))
		if err != nil {
			api.ClusterPanic(err)
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
