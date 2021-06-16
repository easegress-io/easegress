/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://wwwrk.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package worker

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/registrycenter"
)

func (wrk *Worker) nacosAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    meshNacosPrefix + "/ns/instance/list",
			Method:  "GET",
			Handler: wrk.nacosInstanceList,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "POST",
			Handler: wrk.nacosRegister,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "DELETE",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance/beat",
			Method:  "PUT",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "PUT",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshNacosPrefix + "/ns/instance",
			Method:  "GET",
			Handler: wrk.nacosInstance,
		},
		{
			Path:    meshNacosPrefix + "/ns/service/list",
			Method:  "GET",
			Handler: wrk.nacosServiceList,
		},
		{
			Path:    meshNacosPrefix + "/ns/service",
			Method:  "GET",
			Handler: wrk.nacosService,
		},
	}

	return APIs
}

func (wrk *Worker) nacosRegister(w http.ResponseWriter, r *http.Request) {
	err := wrk.registryServer.CheckRegistryURL(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("parse request url parameters failed: %v", err))
		return
	}

	serviceSpec := wrk.service.GetServiceSpec(wrk.serviceName)
	if serviceSpec == nil {
		err := fmt.Errorf("registry to unknown service: %s", wrk.serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	wrk.registryServer.Register(serviceSpec, wrk.ingressServer.Ready, wrk.egressServer.Ready)
}

func (wrk *Worker) nacosInstanceList(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := wrk.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	var serviceInfo *registrycenter.ServiceRegistryInfo

	if serviceInfo, err = wrk.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosSvc := wrk.registryServer.ToNacosService(serviceInfo)

	buff, err := json.Marshal(nacosSvc)
	if err != nil {
		logger.Errorf("json marshal nacosService: %#v err: %v", nacosSvc, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}

func (wrk *Worker) nacosInstance(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := wrk.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	var serviceInfo *registrycenter.ServiceRegistryInfo

	if serviceInfo, err = wrk.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosIns := wrk.registryServer.ToNacosInstanceInfo(serviceInfo)

	buff, err := json.Marshal(nacosIns)
	if err != nil {
		logger.Errorf("json marshal nacosInstance: %#v err: %v", nacosIns, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}

func (wrk *Worker) nacosServiceList(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = wrk.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	serviceList := wrk.registryServer.ToNacosServiceList(serviceInfos)

	buff, err := json.Marshal(serviceList)
	if err != nil {
		logger.Errorf("json marshal serviceList: %#v err: %v", serviceList, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}

func (wrk *Worker) nacosService(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("serviceName")
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("empty serviceName in url parameters"))
		return
	}
	serviceName, err := wrk.registryServer.SplitNacosServiceName(serviceName)
	if err != nil {
		logger.Errorf("nacos invalid servicename: %s", serviceName)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	var serviceInfo *registrycenter.ServiceRegistryInfo
	if serviceInfo, err = wrk.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	nacosSvcDetail := wrk.registryServer.ToNacosServiceDetail(serviceInfo)

	buff, err := json.Marshal(nacosSvcDetail)
	if err != nil {
		logger.Errorf("json marshal nacosSvcDetail: %#v err: %v", nacosSvcDetail, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}
