/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package master provides master role of Easegress for mesh control plane.
package master

import (
	"runtime/debug"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/certmanager"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	defaultDeadRecordExistTime time.Duration = 20 * time.Minute
)

type (
	// Master is the master role of Easegress for mesh control plane.
	Master struct {
		super             *supervisor.Supervisor
		superSpec         *supervisor.Spec
		spec              *spec.Admin
		heartbeatInterval time.Duration
		certManager       *certmanager.CertManager

		store   storage.Storage
		service *service.Service

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
		super:     superSpec.Super(),
		superSpec: superSpec,
		spec:      adminSpec,

		store:   store,
		service: service.New(superSpec),

		done: make(chan struct{}),
	}

	heartbeat, err := time.ParseDuration(m.spec.HeartbeatInterval)
	if err != nil {
		format := "failed to parse heartbeat interval '%s', fallback to default(5s)"
		logger.Errorf(format, m.spec.HeartbeatInterval)
		heartbeat = 5 * time.Second
	}
	m.heartbeatInterval = heartbeat

	m.initMTLS()
	go m.run()

	return m
}

func (m *Master) initMTLS() error {
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

	m.certManager = certmanager.NewCertManager(m.superSpec, m.service, m.spec.Security.CertProvider, appCertTTL, rootCertTTL, m.store)

	return nil
}

func (m *Master) run() {
	ticker := time.NewTicker(m.heartbeatInterval)

	for {
		select {
		case <-m.done:
			ticker.Stop()
			return

		case <-ticker.C:
			if m.needHandle() {
				m.checkServiceInstances()
			}
		}
	}
}

// only handle master routines when it's the cluster leader.
func (m *Master) needHandle() bool {
	return m.superSpec.Super().Cluster().IsLeader()
}

func (m *Master) checkServiceInstances() {
	defer func() {
		if err := recover(); err != nil {
			format := "failed to check instances %v, stack trace: \n%s\n"
			logger.Errorf(format, err, debug.Stack())
		}
	}()

	statuses := m.service.ListAllServiceInstanceStatuses()
	specs := m.service.ListAllServiceInstanceSpecs()

	for _, _spec := range specs {
		if !m.isMeshRegistryName(_spec.RegistryName) {
			continue
		}

		// TODO: improve search performance
		var status *spec.ServiceInstanceStatus
		for _, s := range statuses {
			if s.ServiceName == _spec.ServiceName && s.InstanceID == _spec.InstanceID {
				status = s
			}
		}

		if status == nil {
			format := "status of %s/%s not found, need to delete"
			logger.Warnf(format, _spec.ServiceName, _spec.InstanceID)
			m.deleteInstance(_spec)
			continue
		}

		m.checkLastHeartbeatTime(_spec, status.LastHeartbeatTime)
	}
}

func (m *Master) checkLastHeartbeatTime(_spec *spec.ServiceInstanceSpec, lastHeartbeatTime string) {
	t, err := time.Parse(time.RFC3339, lastHeartbeatTime)
	if err != nil {
		logger.Errorf("BUG: parse last heartbeat time %s failed: %v", lastHeartbeatTime, err)
		return
	}
	gap := time.Since(t)

	// This instance record's time gap is beyond our tolerance, needs to be clean immediately.
	// For freeing storage space
	if gap > defaultDeadRecordExistTime {
		format := "%s/%s expired for %s, need to be deleted"
		logger.Warnf(format, _spec.ServiceName, _spec.InstanceID, gap.String())
		m.deleteInstance(_spec)
		return
	}

	// Bring it down if no heartbeat in 2 heartbeat interval
	if gap > m.heartbeatInterval*2 {
		if _spec.Status != spec.ServiceStatusOutOfService {
			logger.Warnf("%s/%s expired for %s", _spec.ServiceName, _spec.InstanceID, gap.String())
			m.updateInstanceStatus(_spec, spec.ServiceStatusOutOfService)
		}
		return
	}

	if _spec.Status == spec.ServiceStatusOutOfService {
		logger.Infof("%s/%s heartbeat recovered, make it UP", _spec.ServiceName, _spec.InstanceID)
		m.updateInstanceStatus(_spec, spec.ServiceStatusUp)
	}
}

func (m *Master) deleteInstance(_spec *spec.ServiceInstanceSpec) {
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

func (m *Master) isMeshRegistryName(registryName string) bool {
	// NOTE: Empty registry name means it is an internal mesh service by default.
	switch registryName {
	case "", m.superSpec.Name():
		return true
	default:
		return false
	}
}

func (m *Master) updateInstanceStatus(_spec *spec.ServiceInstanceSpec, status string) {
	_spec.Status = status

	buff, err := codectool.MarshalJSON(_spec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", _spec, err)
		return
	}

	key := layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID)
	err = m.store.Put(key, string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

// Close closes the master
func (m *Master) Close() {
	if m.spec.EnablemTLS() {
		m.certManager.Close()
	}
	close(m.done)
}

// Status returns the status of master.
func (m *Master) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}
