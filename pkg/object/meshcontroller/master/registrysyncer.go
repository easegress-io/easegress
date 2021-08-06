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
	"time"

	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	registrySyncer struct {
		superSpec    *supervisor.Spec
		spec         *spec.Admin
		syncInterval time.Duration

		service *service.Service

		done chan struct{}
	}

	// ExternalRegistry is the interface of external service registry.
	ExternalRegistry interface {
		SyncFromExternal()
		SyncToExternal()
		ListExternalServices()
		Notify() <-chan struct{}
	}
)

func newRegistrySyncer(superSpec *supervisor.Spec, registryController string) *registrySyncer {
	spec := superSpec.ObjectSpec().(*spec.Admin)

	rs := &registrySyncer{
		superSpec: superSpec,
		spec:      spec,
		// syncInterval: syncInteral,
		service: service.New(superSpec),
		done:    make(chan struct{}),
	}

	go rs.run()

	return rs
}

func (rs *registrySyncer) run() {
	for {
		select {
		case <-rs.done:
			return
		case <-time.After(rs.syncInterval):
			rs.sync()
		}
	}
}

func (rs *registrySyncer) sync() {

}
