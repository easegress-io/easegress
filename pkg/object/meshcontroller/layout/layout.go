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

	customObjectKindPrefix = "/mesh/custom-object-kinds/"
	customObjectKind       = "/mesh/custom-object-kinds/%s/" // +kind
	allCustomObjectPrefix  = "/mesh/custom-objects/"
	customObjectPrefix     = "/mesh/custom-objects/%s/"    // +kind +name
	customObject           = "/mesh/custom-objects/%s/%s/" // +kind +name

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

// CustomObjectKindPrefix returns the prefix of custom object kinds.
func CustomObjectKindPrefix() string {
	return customObjectKindPrefix
}

// CustomObjectKindKey returns the key of specified custom object kind.
func CustomObjectKindKey(kind string) string {
	return fmt.Sprintf(customObjectKind, kind)
}

// AllCustomObjectPrefix returns the prefix of custom objects.
func AllCustomObjectPrefix() string {
	return allCustomObjectPrefix
}

// CustomObjectPrefix returns the prefix of custom objects.
func CustomObjectPrefix(kind string) string {
	return fmt.Sprintf(customObjectPrefix, kind)
}

// CustomObjectKey returns the key of specified custom object.
func CustomObjectKey(kind, name string) string {
	return fmt.Sprintf(customObject, kind, name)
}
