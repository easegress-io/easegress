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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/v"

	yamljsontool "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshTenantPrefix is the mesh tenant prefix.
	MeshTenantPrefix = "/mesh/tenants"

	// MeshTenantPath is the mesh tenant path.
	MeshTenantPath = "/mesh/tenants/{tenantName:string}"

	// MeshIngressPrefix is the mesh ingress prefix.
	MeshIngressPrefix = "/mesh/ingresses"

	// MeshIngressPath is the mesh ingress path.
	MeshIngressPath = "/mesh/ingresses/{ingressName:string}"

	// MeshServicePrefix is mesh service prefix.
	MeshServicePrefix = "/mesh/services"

	// MeshServicePath is the mesh service path.
	MeshServicePath = "/mesh/services/{serviceName:string}"

	// MeshServiceCanaryPath is the mesh service canary path.
	MeshServiceCanaryPath = "/mesh/services/{serviceName:string}/canary"

	// MeshServiceResiliencePath is the mesh service resilience path.
	MeshServiceResiliencePath = "/mesh/services/{serviceName:string}/resilience"

	// MeshServiceLoadBalancePath is the mesh service load balance path.
	MeshServiceLoadBalancePath = "/mesh/services/{serviceName:string}/loadbalance"

	// MeshServiceOutputServerPath is the mesh service output server path.
	MeshServiceOutputServerPath = "/mesh/services/{serviceName:string}/outputserver"

	// MeshServiceTracingsPath is the mesh service tracings path.
	MeshServiceTracingsPath = "/mesh/services/{serviceName:string}/tracings"

	// MeshServiceMetricsPath is the mesh service metrics path.
	MeshServiceMetricsPath = "/mesh/services/{serviceName:string}/metrics"

	// MeshServiceInstancePrefix is the mesh service prefix.
	MeshServiceInstancePrefix = "/mesh/serviceinstances"

	// MeshServiceInstancePath is the mesh service path.
	MeshServiceInstancePath = "/mesh/serviceinstances/{serviceName:string}/{instanceID:string}"
)

func (m *Master) registerAPIs() {
	meshAPIs := []*api.APIEntry{
		{Path: MeshTenantPrefix, Method: "GET", Handler: m.listTenants},
		{Path: MeshTenantPath, Method: "POST", Handler: m.createTenant},
		{Path: MeshTenantPath, Method: "GET", Handler: m.getTenant},
		{Path: MeshTenantPath, Method: "PUT", Handler: m.updateTenant},
		{Path: MeshTenantPath, Method: "DELETE", Handler: m.deleteTenant},

		{Path: MeshIngressPrefix, Method: "GET", Handler: m.listIngresses},
		{Path: MeshIngressPath, Method: "POST", Handler: m.createIngress},
		{Path: MeshIngressPath, Method: "GET", Handler: m.getIngress},
		{Path: MeshIngressPath, Method: "PUT", Handler: m.updateIngress},
		{Path: MeshIngressPath, Method: "DELETE", Handler: m.deleteIngress},

		{Path: MeshServicePrefix, Method: "GET", Handler: m.listServices},
		{Path: MeshServicePath, Method: "POST", Handler: m.createService},
		{Path: MeshServicePath, Method: "GET", Handler: m.getService},
		{Path: MeshServicePath, Method: "PUT", Handler: m.updateService},
		{Path: MeshServicePath, Method: "DELETE", Handler: m.deleteService},

		// TODO: API to get instances of one service.

		{Path: MeshServiceInstancePrefix, Method: "GET", Handler: m.listServiceInstanceSpecs},
		{Path: MeshServiceInstancePath, Method: "GET", Handler: m.getServiceInstanceSpec},
		{Path: MeshServiceInstancePath, Method: "DELETE", Handler: m.offlineSerivceInstance},

		{Path: MeshServiceCanaryPath, Method: "POST", Handler: m.createPartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "GET", Handler: m.getPartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "PUT", Handler: m.updatePartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "DELETE", Handler: m.deletePartOfService(canaryMeta)},

		{Path: MeshServiceResiliencePath, Method: "POST", Handler: m.createPartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "GET", Handler: m.getPartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "PUT", Handler: m.updatePartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "DELETE", Handler: m.deletePartOfService(resilienceMeta)},

		{Path: MeshServiceLoadBalancePath, Method: "POST", Handler: m.createPartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "GET", Handler: m.getPartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "PUT", Handler: m.updatePartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "DELETE", Handler: m.deletePartOfService(loadBalanceMeta)},

		{Path: MeshServiceOutputServerPath, Method: "POST", Handler: m.createPartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "GET", Handler: m.getPartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "PUT", Handler: m.updatePartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "DELETE", Handler: m.deletePartOfService(outputServerMeta)},

		{Path: MeshServiceTracingsPath, Method: "POST", Handler: m.createPartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "GET", Handler: m.getPartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "PUT", Handler: m.updatePartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "DELETE", Handler: m.deletePartOfService(tracingsMeta)},

		{Path: MeshServiceMetricsPath, Method: "POST", Handler: m.createPartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "GET", Handler: m.getPartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "PUT", Handler: m.updatePartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "DELETE", Handler: m.deletePartOfService(metricsMeta)},
	}

	api.GlobalServer.RegisterAPIs(meshAPIs)
}

func (m *Master) readSpec(w http.ResponseWriter, r *http.Request, spec interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	err = yaml.Unmarshal(body, spec)
	if err != nil {
		return fmt.Errorf("unmarshal %#v to yaml: %v", spec, err)
	}

	vr := v.Validate(spec, body)
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr)
	}

	return nil
}

func (m *Master) convertSpecToPB(spec interface{}, pbSpec interface{}) error {
	buf, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal %#v to json failed: %v", spec, err)
	}

	err = json.Unmarshal(buf, pbSpec)
	if err != nil {
		return fmt.Errorf("unmarshal from json: %s failed: %v", string(buf), err)
	}

	return nil
}

func (m *Master) convertPBToSpec(pbSpec interface{}, spec interface{}) error {
	buf, err := json.Marshal(pbSpec)
	if err != nil {
		return fmt.Errorf("marshal %#v to json: %v", pbSpec, err)
	}

	err = json.Unmarshal(buf, spec)
	if err != nil {
		return fmt.Errorf("unmarshal %#v to spec: %v", spec, err)
	}

	return nil
}

func (m *Master) readAPISpec(w http.ResponseWriter, r *http.Request, pbSpec interface{}, spec interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	err = json.Unmarshal(body, pbSpec)
	if err != nil {
		return fmt.Errorf("unmarshal %s to pb spec %#v failed: %v", string(body), pbSpec, err)
	}

	err = m.convertPBToSpec(pbSpec, spec)
	if err != nil {
		return err
	}

	yamlBuff, err := yamljsontool.JSONToYAML(body)
	if err != nil {
		return err
	}

	vr := v.Validate(spec, yamlBuff)
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr)
	}

	return nil
}
