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
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func (worker *Worker) nacosAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    meshNacosPrefix + "/ns/instance/list",
			Method:  "GET",
			Handler: worker.nacosInstanceList,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "POST",
			Handler: worker.nacosRegister,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "DELETE",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance/beat",
			Method:  "PUT",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "PUT",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "GET",
			Handler: worker.nacosInstance,
		},
		{
			Path:    meshNacosPrefix + "/ns/service/list",
			Method:  "GET",
			Handler: worker.nacosServiceList,
		},
		{
			Path:    meshNacosPrefix + "/ns/service",
			Method:  "GET",
			Handler: worker.nacosService,
		},
	}

	return APIs
}

func (worker *Worker) nacosRegister(w http.ResponseWriter, r *http.Request) {
	err := worker.registryServer.CheckRegistryURL(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("parse request url parameters failed: %v", err))
		return
	}

	serviceSpec := worker.service.GetServiceSpec(worker.serviceName)
	if serviceSpec == nil {
		err := fmt.Errorf("registry to unknown service: %s", worker.serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	worker.registryServer.Register(serviceSpec, worker.ingressServer.Ready, worker.egressServer.Ready)
}

func (worker *Worker) nacosInstanceList(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := worker.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	var serviceInfo *registrycenter.ServiceRegistryInfo

	if serviceInfo, err = worker.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosSvc := worker.registryServer.ToNacosService(serviceInfo)

	buff := codectool.MustMarshalJSON(nacosSvc)
	worker.writeJSONBody(w, buff)
}

func (worker *Worker) nacosInstance(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := worker.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	var serviceInfo *registrycenter.ServiceRegistryInfo

	if serviceInfo, err = worker.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosIns := worker.registryServer.ToNacosInstanceInfo(serviceInfo)

	buff := codectool.MustMarshalJSON(nacosIns)
	worker.writeJSONBody(w, buff)
}

func (worker *Worker) nacosServiceList(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = worker.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	serviceList := worker.registryServer.ToNacosServiceList(serviceInfos)

	buff := codectool.MustMarshalJSON(serviceList)
	worker.writeJSONBody(w, buff)
}

func (worker *Worker) nacosService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := worker.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	var serviceInfo *registrycenter.ServiceRegistryInfo
	if serviceInfo, err = worker.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosSvcDetail := worker.registryServer.ToNacosServiceDetail(serviceInfo)

	buff := codectool.MustMarshalJSON(nacosSvcDetail)
	worker.writeJSONBody(w, buff)
}
