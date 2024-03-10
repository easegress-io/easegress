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

package worker

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
)

const (
	// meshEurekaPrefix is the mesh eureka registry API url prefix.
	meshEurekaPrefix = "/mesh/eureka"

	// meshNacosPrefix is the mesh nacos registry API url prefix.
	meshNacosPrefix = "/nacos/v1"
)

func (worker *Worker) runAPIServer() {
	var apis []*apiEntry
	switch worker.registryType {
	case spec.RegistryTypeConsul:
		apis = worker.consulAPIs()
	case spec.RegistryTypeEureka:
		apis = worker.eurekaAPIs()
	case spec.RegistryTypeNacos:
		apis = worker.nacosAPIs()
	default:
		apis = worker.eurekaAPIs()
	}
	worker.apiServer.registerAPIs(apis)
}

func (worker *Worker) emptyHandler(w http.ResponseWriter, r *http.Request) {
	// EaseMesh does not need to implement some APIS like
	// delete, heartbeat of Eureka/Consul/Nacos.
}

func (worker *Worker) writeJSONBody(w http.ResponseWriter, buff []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}
