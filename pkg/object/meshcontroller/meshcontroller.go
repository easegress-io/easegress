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

// Package meshcontroller provides the service mesh controller.
package meshcontroller

import (
	"strings"

	egapi "github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/api"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/ingresscontroller"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/label"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/master"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/worker"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of MeshController.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of MeshController.
	Kind = "MeshController"
)

func init() {
	supervisor.Register(&MeshController{})
	egapi.RegisterObject(&egapi.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"mesh", "meshcontrollers"},
	})
}

type (
	// MeshController is a business controller to complete EaseMesh.
	MeshController struct {
		superSpec *supervisor.Spec
		spec      *spec.Admin

		api *api.API

		role              string
		master            *master.Master
		worker            *worker.Worker
		ingressController *ingresscontroller.IngressController
	}
)

// Category returns the category of MeshController.
func (mc *MeshController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of MeshController.
func (mc *MeshController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MeshController.
func (mc *MeshController) DefaultSpec() interface{} {
	return &spec.Admin{
		HeartbeatInterval: spec.HeartbeatInterval,
		RegistryType:      spec.RegistryTypeEureka,
		APIPort:           spec.WorkerAPIPort,
		IngressPort:       spec.IngressPort,
	}
}

// Init initializes MeshController.
func (mc *MeshController) Init(superSpec *supervisor.Spec) {
	mc.superSpec, mc.spec = superSpec, superSpec.ObjectSpec().(*spec.Admin)

	mc.reload()
}

// Inherit inherits previous generation of MeshController.
func (mc *MeshController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	mc.superSpec, mc.spec = superSpec, superSpec.ObjectSpec().(*spec.Admin)
	previousGeneration.Close()

	mc.reload()
}

func (mc *MeshController) reload() {
	mc.api = api.New(mc.superSpec)
	meshRole := mc.superSpec.Super().Options().Labels[label.KeyRole]
	serviceName := mc.superSpec.Super().Options().Labels[label.KeyServiceName]

	switch meshRole {
	case "", label.ValueRoleMaster, label.ValueRoleWorker:
		if serviceName == "" {
			meshRole = label.ValueRoleMaster
		} else {
			meshRole = label.ValueRoleWorker
		}
	case label.ValueRoleIngressController:
		// ingress controller does not care about service name
		break
	default:
		logger.Errorf("%s unsupported mesh role: %s (master, worker, ingress-controller)",
			mc.superSpec.Name(), meshRole)
		logger.Infof("%s use default mesh role: master", mc.superSpec.Name())
		meshRole = label.ValueRoleMaster
	}

	switch meshRole {
	case label.ValueRoleMaster:
		logger.Infof("%s running in master role", mc.superSpec.Name())
		mc.role = label.ValueRoleMaster
		mc.master = master.New(mc.superSpec)

	case label.ValueRoleWorker:
		logger.Infof("%s running in worker role", mc.superSpec.Name())
		mc.role = label.ValueRoleWorker
		mc.worker = worker.New(mc.superSpec)

	case label.ValueRoleIngressController:
		logger.Infof("%s running in ingress controller role", mc.superSpec.Name())
		mc.role = label.ValueRoleIngressController
		mc.ingressController = ingresscontroller.New(mc.superSpec)
	}
}

// Status returns the status of MeshController.
func (mc *MeshController) Status() *supervisor.Status {
	if mc.master != nil {
		return mc.master.Status()
	}

	if mc.worker != nil {
		return mc.worker.Status()
	}

	return mc.ingressController.Status()
}

// Close closes MeshController.
func (mc *MeshController) Close() {
	mc.api.Close()

	if mc.master != nil {
		mc.master.Close()
		return
	}

	if mc.worker != nil {
		mc.worker.Close()
		return
	}

	if mc.ingressController != nil {
		mc.ingressController.Close()
		return
	}
}
