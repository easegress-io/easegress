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

package worker

import (
	"github.com/kataras/iris"

	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	// meshEurekaPrefix is the mesh eureka registry API url prefix.
	meshEurekaPrefix = "/mesh/eureka"

	// meshNacosPrefix is the mesh nacos registyr API url prefix.
	meshNacosPrefix = "/nacos/v1"
)

func (w *Worker) runAPIServer() {
	var apis []*apiEntry
	switch w.registryServer.RegistryType {
	case spec.RegistryTypeConsul:
		apis = w.consulAPIs()
	case spec.RegistryTypeEureka:
		apis = w.eurekaAPIs()
	case spec.RegistryTypeNacos:
		apis = w.nacosAPIs()
	default:
		apis = w.eurekaAPIs()
	}
	w.apiServer.registerAPIs(apis)
	go w.apiServer.run()
}

func (w *Worker) emptyHandler(ctx iris.Context) {
	// EaseMesh does not need to implement some APIS like
	// delete, heartbeat of Eureka/Consul/Nacos.
}
