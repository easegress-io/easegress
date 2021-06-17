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
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/registrycenter"
)

func (wrk *Worker) consulAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    "/v1/catalog/register",
			Method:  "PUT",
			Handler: wrk.consulRegister,
		},
		{
			Path:    "/v1/agent/service/register",
			Method:  "PUT",
			Handler: wrk.consulRegister,
		},
		{
			Path:    "/v1/agent/service/deregister",
			Method:  "DELETE",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    "/v1/health/service/{serviceName}",
			Method:  "GET",
			Handler: wrk.healthService,
		},
		{
			Path:    "/v1/catalog/deregister",
			Method:  "DELETE",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    "/v1/catalog/services",
			Method:  "GET",
			Handler: wrk.catalogServices,
		},
		{
			Path:    "/v1/catalog/service/{serviceName}",
			Method:  "GET",
			Handler: wrk.catalogService,
		},
	}

	return APIs
}

func (wrk *Worker) consulRegister(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := w.Header().Get("Content-Type")

	if err := wrk.registryServer.CheckRegistryBody(contentType, body); err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
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

func (wrk *Worker) healthService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = wrk.registryServer.DiscoveryService(serviceName); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	serviceEntry := wrk.registryServer.ToConsulHealthService(serviceInfo)

	buff, err := json.Marshal(serviceEntry)
	if err != nil {
		logger.Errorf("json marshal sericeEntry: %#v err: %v", serviceEntry, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}

func (wrk *Worker) catalogService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = wrk.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	catalogService := wrk.registryServer.ToConsulCatalogService(serviceInfo)

	buff, err := json.Marshal(catalogService)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		logger.Errorf("json marshal catalogService: %#v err: %v", catalogService, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}

func (wrk *Worker) catalogServices(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = wrk.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	catalogServices := wrk.registryServer.ToConsulServices(serviceInfos)

	buff, err := json.Marshal(catalogServices)
	if err != nil {
		logger.Errorf("json marshal catalogServices: %#v err: %v", catalogServices, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", registrycenter.ContentTypeJSON)
	w.Write(buff)
}
