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

package api

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

type servicesByOrder []*spec.Service

func (s servicesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s servicesByOrder) Len() int           { return len(s) }
func (s servicesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type configMapsByOrder []*v1.ConfigMap

func (s configMapsByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s configMapsByOrder) Len() int           { return len(s) }
func (s configMapsByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type secretsByOrder []*v1.Secret

func (s secretsByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s secretsByOrder) Len() int           { return len(s) }
func (s secretsByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (a *API) readServiceName(r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

func (a *API) listServices(w http.ResponseWriter, r *http.Request) {
	specs := a.service.ListServiceSpecs()

	sort.Sort(servicesByOrder(specs))

	apiSpecs := make([]*v2alpha1.Service, 0, len(specs))
	for _, v := range specs {
		service := &v2alpha1.Service{}
		err := a.convertSpecToPB(v, service)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, service)
	}

	buff := codectool.MustMarshalJSON(apiSpecs)
	a.writeJSONBody(w, buff)
}

func (a *API) createService(w http.ResponseWriter, r *http.Request) {
	pbServiceSpec := &v2alpha1.Service{}
	serviceSpec := &spec.Service{}

	err := a.readAPISpec(r, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceSpec.Name)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", serviceSpec.Name))
		return
	}

	tenantSpec := a.service.GetTenantSpec(serviceSpec.RegisterTenant)
	if tenantSpec == nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
		return
	}

	tenantSpec.Services = append(tenantSpec.Services, serviceSpec.Name)

	a.service.PutServiceSpec(serviceSpec)
	a.service.PutTenantSpec(tenantSpec)

	w.Header().Set("Location", path.Join(r.URL.Path, serviceSpec.Name))
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getService(w http.ResponseWriter, r *http.Request) {
	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	serviceSpec := a.service.GetServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	pbServiceSpec := &v2alpha1.Service{}
	err = a.convertSpecToPB(serviceSpec, pbServiceSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", serviceSpec, err))
	}

	buff := codectool.MustMarshalJSON(pbServiceSpec)
	a.writeJSONBody(w, buff)
}

func (a *API) updateService(w http.ResponseWriter, r *http.Request) {
	pbServiceSpec := &v2alpha1.Service{}
	serviceSpec := &spec.Service{}

	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = a.readAPISpec(r, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	if serviceSpec.RegisterTenant != oldSpec.RegisterTenant {
		newTenantSpec := a.service.GetTenantSpec(serviceSpec.RegisterTenant)
		if newTenantSpec == nil {
			api.HandleAPIError(w, r, http.StatusBadRequest,
				fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
			return
		}
		newTenantSpec.Services = append(newTenantSpec.Services, serviceSpec.Name)

		oldTenantSpec := a.service.GetTenantSpec(oldSpec.RegisterTenant)
		if oldTenantSpec == nil {
			panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
		}
		oldTenantSpec.Services = stringtool.DeleteStrInSlice(oldTenantSpec.Services, serviceName)

		a.service.PutTenantSpec(newTenantSpec)
		a.service.PutTenantSpec(oldTenantSpec)
	}

	a.service.PutServiceSpec(serviceSpec)
}

func (a *API) deleteService(w http.ResponseWriter, r *http.Request) {
	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	tenantSpec := a.service.GetTenantSpec(oldSpec.RegisterTenant)
	if tenantSpec == nil {
		panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
	}

	tenantSpec.Services = stringtool.DeleteStrInSlice(tenantSpec.Services, serviceName)

	a.service.PutTenantSpec(tenantSpec)
	a.service.DeleteServiceSpec(serviceName)
}

func (a *API) getServiceDeployment(w http.ResponseWriter, r *http.Request) {
	const annotationServiceNameKey = "mesh.megaease.com/service-name"

	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if a.k8sClient == nil {
		api.HandleAPIError(w, r, http.StatusServiceUnavailable,
			fmt.Errorf("k8s client not found"))
		return
	}

	serviceDeployment := &spec.ServiceDeployment{}

	deployments, err := a.k8sClient.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		api.HandleAPIError(w, r, http.StatusServiceUnavailable, err)
		return
	}

	for _, deployment := range deployments.Items {
		if strings.HasPrefix(deployment.Name, "canary-") {
			continue
		}

		if deployment.Annotations[annotationServiceNameKey] == serviceName {
			serviceDeployment.App = deployment
			serviceDeployment.ConfigMaps, serviceDeployment.Secrets = a.getConfigMapsAndSecrets(&deployment)
			a.writeJSONBody(w, codectool.MustMarshalJSON(serviceDeployment))
			return
		}
	}

	statefulsets, _ := a.k8sClient.AppsV1().StatefulSets("").List(context.Background(), metav1.ListOptions{})
	for _, statefulset := range statefulsets.Items {
		if statefulset.Annotations[annotationServiceNameKey] == serviceName {
			serviceDeployment.App = statefulset
			serviceDeployment.ConfigMaps, serviceDeployment.Secrets = a.getConfigMapsAndSecrets(&statefulset)
			a.writeJSONBody(w, codectool.MustMarshalJSON(serviceDeployment))
			return
		}
	}

	api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s deployment spec not found", serviceName))
}

func (a *API) getConfigMapsAndSecrets(spec interface{}) ([]*corev1.ConfigMap, []*corev1.Secret) {
	var namespace string
	var volumes []corev1.Volume
	switch spec := spec.(type) {
	case *appsv1.Deployment:
		namespace = spec.Namespace
		volumes = spec.Spec.Template.Spec.Volumes
	case *appsv1.StatefulSet:
		namespace = spec.Namespace
		volumes = spec.Spec.Template.Spec.Volumes
	default:
		panic(fmt.Errorf("unknown spec type %T", spec))
	}

	configMaps := map[string]*corev1.ConfigMap{}
	secrets := map[string]*corev1.Secret{}
	for _, volume := range volumes {
		if volume.ConfigMap != nil {
			configMap, err := a.k8sClient.CoreV1().ConfigMaps(namespace).Get(context.Background(),
				volume.ConfigMap.Name, metav1.GetOptions{})
			if err != nil {
				api.ClusterPanic(fmt.Errorf("get configmap %s failed: %v", volume.ConfigMap.Name, err))
			}

			configMaps[configMap.Name] = configMap
		}
		if volume.Secret != nil {
			secret, err := a.k8sClient.CoreV1().Secrets(namespace).Get(context.Background(),
				volume.Secret.SecretName, metav1.GetOptions{})
			if err != nil {
				api.ClusterPanic(fmt.Errorf("get secret %s failed: %v", volume.Secret.SecretName, err))
			}
			secrets[secret.Name] = secret
		}
	}

	cms := []*corev1.ConfigMap{}
	for _, cm := range configMaps {
		cms = append(cms, cm)
	}
	sort.Sort(configMapsByOrder(cms))

	ss := []*corev1.Secret{}
	for _, s := range secrets {
		ss = append(ss, s)
	}
	sort.Sort(secretsByOrder(ss))

	return cms, ss
}
