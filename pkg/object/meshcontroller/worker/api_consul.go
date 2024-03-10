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
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func (worker *Worker) consulAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    "/v1/catalog/register",
			Method:  "PUT",
			Handler: worker.consulRegister,
		},
		{
			Path:    "/v1/agent/service/register",
			Method:  "PUT",
			Handler: worker.consulRegister,
		},
		{
			Path:    "/v1/agent/service/deregister",
			Method:  "DELETE",
			Handler: worker.emptyHandler,
		},
		{
			Path:    "/v1/health/service/{serviceName}",
			Method:  "GET",
			Handler: worker.healthService,
		},
		{
			Path:    "/v1/catalog/deregister",
			Method:  "DELETE",
			Handler: worker.emptyHandler,
		},
		{
			Path:    "/v1/catalog/services",
			Method:  "GET",
			Handler: worker.catalogServices,
		},
		{
			Path:    "/v1/catalog/service/{serviceName}",
			Method:  "GET",
			Handler: worker.catalogService,
		},
	}

	return APIs
}

func (worker *Worker) consulRegister(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := w.Header().Get("Content-Type")

	if err := worker.registryServer.CheckRegistryBody(contentType, body); err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
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

func (worker *Worker) healthService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = worker.registryServer.DiscoveryService(serviceName); err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	serviceEntry := worker.registryServer.ToConsulHealthService(serviceInfo)

	buff := codectool.MustMarshalJSON(serviceEntry)
	worker.writeJSONBody(w, buff)
}

func (worker *Worker) catalogService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = worker.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	catalogService := worker.registryServer.ToConsulCatalogService(serviceInfo)

	buff := codectool.MustMarshalJSON(catalogService)
	worker.writeJSONBody(w, buff)
}

func (worker *Worker) catalogServices(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = worker.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	catalogServices := worker.registryServer.ToConsulServices(serviceInfos)

	buff := codectool.MustMarshalJSON(catalogServices)
	worker.writeJSONBody(w, buff)
}
