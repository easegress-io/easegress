/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package master

import (
	"runtime/debug"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/certmanager"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	defaultCleanInterval       time.Duration = 10 * time.Minute
	defaultDeadRecordExistTime time.Duration = 20 * time.Minute
)

type (
	// Master is the master role of Easegress for mesh control plane.
	Master struct {
		super               *supervisor.Supervisor
		superSpec           *supervisor.Spec
		spec                *spec.Admin
		maxHeartbeatTimeout time.Duration
		certMananger        *certmanager.CertManager

		registrySyncer *registrySyncer
		store          storage.Storage
		service        *service.Service

		done chan struct{}
	}

	// Status is the status of mesh master.
	Status struct{}
)

// New creates a mesh master.
func New(superSpec *supervisor.Spec) *Master {
	store := storage.New(superSpec.Name(), superSpec.Super().Cluster())
	adminSpec := superSpec.ObjectSpec().(*spec.Admin)

	m := &Master{
		superSpec: superSpec,
		spec:      adminSpec,

		store:          store,
		service:        service.New(superSpec),
		registrySyncer: newRegistrySyncer(superSpec),

		done: make(chan struct{}),
	}

	heartbeat, err := time.ParseDuration(m.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval %s to duration failed: %v",
			m.spec.HeartbeatInterval, err)
	}
	m.maxHeartbeatTimeout = heartbeat * 2

	m.run()

	return m
}

func (m *Master) securityRoutine() error {
	if !m.spec.EnablemTLS() {
		return nil
	}
	appCertTTL, err := time.ParseDuration(m.spec.Security.AppCertTTL)
	if err != nil {
		logger.Errorf("BUG: parse app cert ttl: %s failed: %v", appCertTTL, err)
		return err
	}
	rootCertTTL, err := time.ParseDuration(m.spec.Security.RootCertTTL)
	if err != nil {
		logger.Errorf("BUG: parse root cert ttl: %s failed: %v", rootCertTTL, err)
		return err
	}

	m.certMananger = certmanager.NewCertManager(m.superSpec, m.service, m.spec.Security.CertProvider, appCertTTL, rootCertTTL, m.store)

	return nil
}

func (m *Master) run() {
	watchInterval, err := time.ParseDuration(m.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			m.spec.HeartbeatInterval, err)
		return
	}

	if err := m.securityRoutine(); err != nil {
		logger.Errorf("start security routine failed, err: %v", err)
		return
	}

	go m.checkHeartbeat(watchInterval)
	go m.clean()

}

// only handle master routines when its the cluster leader.
func (m *Master) needHandle() bool {
	return m.superSpec.Super().Cluster().IsLeader()
}

func (m *Master) checkHeartbeat(watchInterval time.Duration) {
	for {
		select {
		case <-m.done:
			return
		case <-time.After(watchInterval):
			if m.needHandle() {
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
}

func (m *Master) clean() {
	for {
		select {
		case <-m.done:
			return
		case <-time.After(defaultCleanInterval):
			if m.needHandle() {
				func() {
					defer func() {
						if err := recover(); err != nil {
							logger.Errorf("failed to clean dead instances: %v, stack trace: \n%s\n",
								err, debug.Stack())
						}
					}()
					m.cleanDeadInstances()
				}()
			}
		}
	}
}

func (m *Master) scanInstances() (failedInstances []*spec.ServiceInstanceSpec,
	rebornInstances []*spec.ServiceInstanceSpec, deadInstances []*spec.ServiceInstanceSpec) {

	statuses := m.service.ListAllServiceInstanceStatuses()
	specs := m.service.ListAllServiceInstanceSpecs()

	now := time.Now()
	for _, _spec := range specs {
		if !m.isMeshRegistryName(_spec.RegistryName) {
			continue
		}

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
				// This instance record's time gap is beyond our tolerance, needs to be clean immediately.
				// For freeing storage space
				if gap > defaultDeadRecordExistTime {
					logger.Errorf("%s/%s expired for %s, need to be deleted", _spec.ServiceName, _spec.InstanceID, gap.String())
					deadInstances = append(deadInstances, _spec)
				} else if _spec.Status != spec.ServiceStatusOutOfService {
					logger.Errorf("%s/%s expired for %s", _spec.ServiceName, _spec.InstanceID, gap.String())
					failedInstances = append(failedInstances, _spec)
				}
			} else {
				if _spec.Status == spec.ServiceStatusOutOfService {
					logger.Infof("%s/%s heartbeat recovered, make it UP", _spec.ServiceName, _spec.InstanceID)
					rebornInstances = append(rebornInstances, _spec)
				}
			}
		} else {
			logger.Errorf("status of %s/%s not found, need to delete", _spec.ServiceName, _spec.InstanceID)
			failedInstances = append(failedInstances, _spec)
			deadInstances = append(deadInstances, _spec)
		}
	}
	return
}

func (m *Master) checkInstancesHeartbeat() {
	failedInstances, rebornInstances, _ := m.scanInstances()
	m.handleFailedInstances(failedInstances)
	m.handleRebornInstances(rebornInstances)
}

func (m *Master) cleanDeadInstances() {
	_, _, deadInstances := m.scanInstances()
	for _, _spec := range deadInstances {
		specKey := layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID)
		err := m.store.Delete(specKey)
		if err != nil {
			api.ClusterPanic(err)
		} else {
			logger.Infof("clean instance spec: %s", specKey)
		}

		statusKey := layout.ServiceInstanceStatusKey(_spec.ServiceName, _spec.InstanceID)
		if err = m.store.Delete(statusKey); err != nil {
			api.ClusterPanic(err)
		} else {
			logger.Infof("clean instance status: %s", statusKey)
		}
	}
}

func (m *Master) isMeshRegistryName(registryName string) bool {
	// NOTE: Empty registry name means it is an internal mesh service by default.
	switch registryName {
	case "", m.superSpec.Name():
		return true
	default:
		return false
	}
}

func (m *Master) handleRebornInstances(rebornInstances []*spec.ServiceInstanceSpec) {
	m.updateInstanceStatus(rebornInstances, spec.ServiceStatusUp)
}

func (m *Master) handleFailedInstances(failedInstances []*spec.ServiceInstanceSpec) {
	m.updateInstanceStatus(failedInstances, spec.ServiceStatusOutOfService)
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
	if m.spec.EnablemTLS() {
		m.certMananger.Close()
	}
	close(m.done)
}

// Status returns the status of master.
func (m *Master) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}
