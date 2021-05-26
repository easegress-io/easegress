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
	"net/http"
	"reflect"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/util/stringtool"
	v1alpha1 "github.com/megaease/easemesh-api/v1alpha1"
)

type servicesByOrder []*spec.Service

func (s servicesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s servicesByOrder) Len() int           { return len(s) }
func (s servicesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readServiceName(ctx iris.Context) (string, error) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

func (m *Master) listServices(ctx iris.Context) {
	specs := m.service.ListServiceSpecs()

	sort.Sort(servicesByOrder(specs))

	var apiSpecs []*v1alpha1.Service
	for _, v := range specs {
		service := &v1alpha1.Service{}
		err := m.convertSpecToPB(v, service)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, service)
	}

	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "application/json")
	ctx.Write(buff)
}

func (m *Master) createService(ctx iris.Context) {
	pbServiceSpec := &v1alpha1.Service{}
	serviceSpec := &spec.Service{}

	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(ctx, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec != nil {
		api.HandleAPIError(ctx, http.StatusConflict, fmt.Errorf("%s existed", serviceName))
		return
	}

	tenantSpec := m.service.GetTenantSpec(serviceSpec.RegisterTenant)
	if tenantSpec == nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
		return
	}

	tenantSpec.Services = append(tenantSpec.Services, serviceSpec.Name)

	m.service.PutServiceSpec(serviceSpec)
	m.service.PutTenantSpec(tenantSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) getService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := m.service.GetServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	pbServiceSpec := &v1alpha1.Service{}
	err = m.convertSpecToPB(serviceSpec, pbServiceSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", serviceSpec, err))
	}

	buff, err := json.Marshal(pbServiceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", serviceSpec, err))
	}

	ctx.Header("Content-Type", "application/json")
	ctx.Write(buff)
}

func (m *Master) updateService(ctx iris.Context) {
	pbServiceSpec := &v1alpha1.Service{}
	serviceSpec := &spec.Service{}

	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readAPISpec(ctx, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	if serviceSpec.RegisterTenant != oldSpec.RegisterTenant {
		newTenantSpec := m.service.GetTenantSpec(serviceSpec.RegisterTenant)
		if newTenantSpec == nil {
			api.HandleAPIError(ctx, http.StatusBadRequest,
				fmt.Errorf("tenant %s not found", serviceName))
			return
		}
		newTenantSpec.Services = append(newTenantSpec.Services, serviceSpec.Name)

		oldTenantSpec := m.service.GetTenantSpec(oldSpec.RegisterTenant)
		if oldTenantSpec == nil {
			panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
		}
		oldTenantSpec.Services = stringtool.DeleteStrInSlice(oldTenantSpec.Services, serviceName)

		m.service.PutTenantSpec(newTenantSpec)
		m.service.PutTenantSpec(oldTenantSpec)
	}

	globalCanaryHeaders := m.service.GetGlobalCanaryHeaders()
	uniqueHeaders := serviceSpec.UniqueCanaryHeaders()
	oldUniqueHeaders := oldSpec.UniqueCanaryHeaders()

	if !reflect.DeepEqual(uniqueHeaders, oldUniqueHeaders) {
		if globalCanaryHeaders == nil {
			globalCanaryHeaders = &spec.GlobalCanaryHeaders{
				ServiceHeaders: map[string][]string{},
			}
		}
		globalCanaryHeaders.ServiceHeaders[serviceName] = uniqueHeaders
		m.service.PutGlobalCanaryHeaders(globalCanaryHeaders)
	}

	m.service.PutServiceSpec(serviceSpec)
}

func (m *Master) deleteService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	tenantSpec := m.service.GetTenantSpec(oldSpec.RegisterTenant)
	if tenantSpec == nil {
		panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
	}

	tenantSpec.Services = stringtool.DeleteStrInSlice(tenantSpec.Services, serviceName)

	m.service.PutTenantSpec(tenantSpec)
	m.service.DeleteServiceSpec(serviceName)
}
