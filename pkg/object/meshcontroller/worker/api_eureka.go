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
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/go-chi/chi/v5"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/registrycenter"
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

func (wrk *Worker) eurekaAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}",
			Method:  "POST",
			Handler: wrk.eurekaRegister,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "DELETE",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "PUT",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/",
			Method:  "GET",
			Handler: wrk.apps,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}",
			Method:  "GET",
			Handler: wrk.app,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}",
			Method:  "GET",
			Handler: wrk.getAppInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/instances/{instanceID}",
			Method:  "GET",
			Handler: wrk.getInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/status",
			Method:  "PUT",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/status",
			Method:  "DELETE",
			Handler: wrk.emptyHandler,
		}, {
			Path:    meshEurekaPrefix + "/apps/{serviceName}/{instanceID}/metadata",
			Method:  "PUT",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/vips/{vipAddress}",
			Method:  "GET",
			Handler: wrk.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/svips/{svipAddress}",
			Method:  "GET",
			Handler: wrk.emptyHandler,
		},
	}

	return APIs
}

func (wrk *Worker) detectedAccept(accept string) string {
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

func (wrk *Worker) eurekaRegister(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := r.Header.Get("Content-Type")
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

	// NOTE: According to eureka APIs list:
	// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
	w.WriteHeader(http.StatusNoContent)
}

func (wrk *Worker) apps(w http.ResponseWriter, r *http.Request) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = wrk.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	xmlAPPs := wrk.registryServer.ToEurekaApps(serviceInfos)
	jsonAPPs := eurekaJSONApps{
		APPs: eurekaAPPs{
			VersionDelta: strconv.Itoa(xmlAPPs.VersionsDelta),
			AppHashCode:  xmlAPPs.AppsHashcode,
		},
	}

	for _, v := range xmlAPPs.Applications {
		jsonAPPs.APPs.Application = append(jsonAPPs.APPs.Application, eurekaAPP{Name: v.Name, Instances: v.Instances})
	}

	accept := wrk.detectedAccept(r.Header.Get("Accept"))

	rsp, err := wrk.encodByAcceptType(accept, jsonAPPs, xmlAPPs)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", accept)
	w.Write([]byte(rsp))
}

func (wrk *Worker) app(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		api.HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("empty service name(app)"))
		return
	}

	// eureka use 'delta' after /apps/, need to handle this
	// special case here.
	if serviceName == "delta" {
		wrk.apps(w, r)
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
	accept := wrk.detectedAccept(r.Header.Get("Accept"))
	xmlAPP := wrk.registryServer.ToEurekaApp(serviceInfo)

	jsonApp := eurekaJSONAPP{
		APP: eurekaAPP{
			Name:      xmlAPP.Name,
			Instances: xmlAPP.Instances,
		},
	}
	rsp, err := wrk.encodByAcceptType(accept, jsonApp, xmlAPP)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", accept)
	w.Write([]byte(rsp))
}

func (wrk *Worker) getAppInstance(w http.ResponseWriter, r *http.Request) {
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

	serviceInfo, err := wrk.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}

	if serviceInfo.Service.Name == serviceName && instanceID == serviceInfo.Ins.InstanceID {
		ins := wrk.registryServer.ToEurekaInstanceInfo(serviceInfo)
		accept := wrk.detectedAccept(w.Header().Get("Accept"))

		rsp, err := wrk.encodByAcceptType(accept, ins, ins)
		if err != nil {
			logger.Errorf("encode accept: %s failed: %v", accept, err)
			api.HandleAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", accept)
		w.Write([]byte(rsp))
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (wrk *Worker) getInstance(w http.ResponseWriter, r *http.Request) {
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

	serviceInfo, err := wrk.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	ins := wrk.registryServer.ToEurekaInstanceInfo(serviceInfo)
	accept := wrk.detectedAccept(r.Header.Get("Accept"))

	rsp, err := wrk.encodByAcceptType(accept, ins, ins)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", accept)
	w.Write([]byte(rsp))
}

func (wrk *Worker) encodByAcceptType(accept string, jsonSt interface{}, xmlSt interface{}) ([]byte, error) {
	switch accept {
	case registrycenter.ContentTypeJSON:
		buff := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buff)
		err := enc.Encode(jsonSt)
		return buff.Bytes(), err
	default:
		buff, err := xml.Marshal(xmlSt)
		return buff, err
	}
}
