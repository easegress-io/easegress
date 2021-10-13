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

package layout

import (
	"fmt"
)

const (
	serviceSpecPrefix = "/mesh/service-spec/"
	serviceSpec       = "/mesh/service-spec/%s" // +serviceName

	allServiceInstanceSpecPrefix   = "/mesh/service-instances/spec/"
	allServiceInstanceStatusPrefix = "/mesh/service-instances/status/"
	serviceInstanceSpecPrefix      = "/mesh/service-instances/spec/%s/"     // +serviceName
	serviceInstanceStatusPrefix    = "/mesh/service-instances/status/%s/"   // +serviceName
	serviceInstanceSpec            = "/mesh/service-instances/spec/%s/%s"   // +serviceName +instanceID
	serviceInstanceStatus          = "/mesh/service-instances/status/%s/%s" // +serviceName +instanceID

	tenant       = "/mesh/tenants/%s" // +tenantName
	tenantPrefix = "/mesh/tenants/"

	ingress       = "/mesh/ingress/%s" // + ingressName
	ingressPrefix = "/mesh/ingress/"

	serviceCert           = "/mesh/cert/service-cert/%s" // + ServiceName
	allServiceCertPrefix  = "/mesh/cert/service-cert/"
	rootCert              = "/mesh/cert/root-cert"
	ingressControllerCert = "/mesh/cert/ingress-controller-cert"

	customResourceKindPrefix = "/mesh/custom-resource-kinds/"
	customResourceKind       = "/mesh/custom-resource-kinds/%s/" // +kind
	allCustomResourcePrefix  = "/mesh/custom-resources/"
	customResourcePrefix     = "/mesh/custom-resources/%s/"    // +kind
	customResource           = "/mesh/custom-resources/%s/%s/" // +kind +name

	globalCanaryHeaders = "/mesh/canary-headers"
)

// ServiceSpecPrefix returns the prefix of service.
func ServiceSpecPrefix() string {
	return serviceSpecPrefix
}

// ServiceSpecKey returns the key of service spec.
func ServiceSpecKey(serviceName string) string {
	return fmt.Sprintf(serviceSpec, serviceName)
}

// ServiceInstanceSpecKey returns the key of service instance spec.
func ServiceInstanceSpecKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceSpec, serviceName, instanceID)
}

// ServiceInstanceStatusKey returns the key of service instance status.
func ServiceInstanceStatusKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceStatus, serviceName, instanceID)
}

// ServiceInstanceSpecPrefix returns the prefix of service instance specs.
func ServiceInstanceSpecPrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstanceSpecPrefix, serviceName)
}

// ServiceInstanceStatusPrefix returns the prefix of service instance statuses.
func ServiceInstanceStatusPrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstanceStatusPrefix, serviceName)
}

// AllServiceInstanceSpecPrefix returns the prefix of all service instance specs.
func AllServiceInstanceSpecPrefix() string {
	return allServiceInstanceSpecPrefix
}

// AllServiceInstanceStatusPrefix returns the prefix of all service instance statuses.
func AllServiceInstanceStatusPrefix() string {
	return allServiceInstanceStatusPrefix
}

// TenantSpecKey returns the key of tenant spec.
func TenantSpecKey(t string) string {
	return fmt.Sprintf(tenant, t)
}

// TenantPrefix returns the prefix of tenant.
func TenantPrefix() string {
	return tenantPrefix
}

// IngressSpecKey returns the key of ingress spec.
func IngressSpecKey(t string) string {
	return fmt.Sprintf(ingress, t)
}

// IngressPrefix returns the prefix of ingress.
func IngressPrefix() string {
	return ingressPrefix
}

// GlobalCanaryHeaders returns the key of global service's canary headers.
func GlobalCanaryHeaders() string {
	return globalCanaryHeaders
}

// CustomResourceKindPrefix returns the prefix of custom object kinds.
func CustomResourceKindPrefix() string {
	return customResourceKindPrefix
}

// CustomResourceKindKey returns the key of specified custom object kind.
func CustomResourceKindKey(kind string) string {
	return fmt.Sprintf(customResourceKind, kind)
}

// AllCustomResourcePrefix returns the prefix of custom objects.
func AllCustomResourcePrefix() string {
	return allCustomResourcePrefix
}

// CustomResourcePrefix returns the prefix of custom objects.
func CustomResourcePrefix(kind string) string {
	return fmt.Sprintf(customResourcePrefix, kind)
}

// CustomResourceKey returns the key of specified custom object.
func CustomResourceKey(kind, name string) string {
	return fmt.Sprintf(customResource, kind, name)
}

// ServiceCertKey returns the key of specified service's cert.
func ServiceCertKey(name string) string {
	return fmt.Sprintf(serviceCert, name)
}

// AllServiceCertPrefix returns the prefix of all service's cert.
func AllServiceCertPrefix() string {
	return allServiceCertPrefix
}

// RootCertKey returns the root cert key.
func RootCertKey() string {
	return rootCert
}

// IngressControllerCertKey returns the ingress controller cert key.
func IngressControllerCertKey() string {
	return ingressControllerCert
}
