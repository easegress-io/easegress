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
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	eurekaJSONApps struct {
		APPs eurekaAPPs `json:"applications"`
	}

	eurekaAPPs struct {
		VersionDelta string      `json:"versions__delta"`
		AppHashCode  string      `json:"apps__hashcode"`
		Application  []eurekaAPP `json:"application"`
	}

	eurekaJSONAPP struct {
		APP eurekaAPP `json:"application"`
	}

	eurekaAPP struct {
		Name      string                `json:"name"`
		Instances []eureka.InstanceInfo `json:"instance"`
	}
)

func (worker *Worker) eurekaAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}",
			Method:  "POST",
			Handler: worker.eurekaRegister,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "DELETE",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "PUT",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/",
			Method:  "GET",
			Handler: worker.apps,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}",
			Method:  "GET",
			Handler: worker.app,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "GET",
			Handler: worker.getAppInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/instances/{instanceID}",
			Method:  "GET",
			Handler: worker.getInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/status",
			Method:  "PUT",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/status",
			Method:  "DELETE",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/metadata",
			Method:  "PUT",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/vips/{vipAddress}",
			Method:  "GET",
			Handler: worker.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/svips/{svipAddress}",
			Method:  "GET",
			Handler: worker.emptyHandler,
		},
	}

	return APIs
}

func (worker *Worker) detectedAccept(accept string) string {
	accepts := strings.Split(accept, ",")

	for _, v := range accepts {
		if v == registrycenter.ContentTypeJSON {
			return registrycenter.ContentTypeJSON
		}
		if v == registrycenter.ContentTypeXML {
			return registrycenter.ContentTypeXML
		}
	}

	return registrycenter.ContentTypeXML
}

func (worker *Worker) eurekaRegister(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := r.Header.Get("Content-Type")
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

	// NOTE: According to eureka APIs list:
	// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
	w.WriteHeader(http.StatusNoContent)
}

func (worker *Worker) apps(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = worker.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	xmlAPPs := worker.registryServer.ToEurekaApps(serviceInfos)
	jsonAPPs := eurekaJSONApps{
		APPs: eurekaAPPs{
			VersionDelta: strconv.Itoa(xmlAPPs.VersionsDelta),
			AppHashCode:  xmlAPPs.AppsHashcode,
		},
	}

	for _, v := range xmlAPPs.Applications {
		jsonAPPs.APPs.Application = append(jsonAPPs.APPs.Application, eurekaAPP{Name: v.Name, Instances: v.Instances})
	}

	accept := worker.detectedAccept(r.Header.Get("Accept"))

	rsp, err := worker.encodeByAcceptType(accept, jsonAPPs, xmlAPPs)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", accept)
	w.Write(rsp)
}

func (worker *Worker) app(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name(app)"))
		return
	}

	// eureka use 'delta' after /apps/, need to handle this
	// special case here.
	if serviceName == "delta" {
		worker.apps(w, r)
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
	accept := worker.detectedAccept(r.Header.Get("Accept"))
	xmlAPP := worker.registryServer.ToEurekaApp(serviceInfo)

	jsonApp := eurekaJSONAPP{
		APP: eurekaAPP{
			Name:      xmlAPP.Name,
			Instances: xmlAPP.Instances,
		},
	}
	rsp, err := worker.encodeByAcceptType(accept, jsonApp, xmlAPP)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", accept)
	w.Write(rsp)
}

func (worker *Worker) getAppInstance(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name(app)"))
		return
	}
	instanceID := chi.URLParam(r, "instanceID")
	if instanceID == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty instanceID"))
		return
	}

	serviceInfo, err := worker.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	if serviceInfo.Service.Name == serviceName && instanceID == serviceInfo.Ins.InstanceID {
		ins := worker.registryServer.ToEurekaInstanceInfo(serviceInfo)
		accept := worker.detectedAccept(w.Header().Get("Accept"))

		rsp, err := worker.encodeByAcceptType(accept, ins, ins)
		if err != nil {
			logger.Errorf("encode accept: %s failed: %v", accept, err)
			api.HandleAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", accept)
		w.Write(rsp)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (worker *Worker) getInstance(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instanceID")
	if instanceID == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty instanceID"))
		return
	}
	serviceName := registrycenter.GetServiceName(instanceID)
	if len(serviceName) == 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("unknown instanceID: %s", instanceID))
		return
	}

	serviceInfo, err := worker.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	ins := worker.registryServer.ToEurekaInstanceInfo(serviceInfo)
	accept := worker.detectedAccept(r.Header.Get("Accept"))

	rsp, err := worker.encodeByAcceptType(accept, ins, ins)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", accept)
	w.Write(rsp)
}

func (worker *Worker) encodeByAcceptType(accept string, jsonSt interface{}, xmlSt interface{}) ([]byte, error) {
	switch accept {
	case registrycenter.ContentTypeJSON:
		return codectool.MarshalJSON(jsonSt)
	default:
		return xml.Marshal(xmlSt)
	}
}
