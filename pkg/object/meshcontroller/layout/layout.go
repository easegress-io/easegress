package layout

import (
	"fmt"
)

const (
	serviceSpecFormat = "/mesh/services/%s/spec" // +serviceName

	serviceInstancePrefixFormat    = "/mesh/services/%s/instances"    // +serviceName
	serviceInstanceFormat          = "/mesh/services/%s/instances/%s" // +serviceName +instanceID
	serviceInstanceHeartbeatFormat = "/mesh/services/%s/heartbeat/%s" // +serviceName +instanceID

	tenantFormat = "/mesh/tenants/%s" // +tenantName
	tenantPrefix = "/mesh/tenants"
)

// ServiceKey returns serivce key.
func ServiceKey(serviceName string) string {
	return fmt.Sprintf(serviceSpecFormat, serviceName)
}

// ServiceInstanceKey returns service instance key.
func ServiceInstanceKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceFormat, serviceName, instanceID)
}

// ServiceInstancePrefix returns prefix of the serivce instances.
func ServiceInstancePrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstancePrefixFormat, serviceName)
}

// ServiceHeartbeatKey returns service instance hearbeat key.
func ServiceHeartbeatKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceFormat, serviceName, instanceID)
}

// TenantKey returns tenant key.
func TenantKey(tenant string) string {
	return fmt.Sprintf(tenantFormat, tenant)
}

// TenantPrefix returns tenant prefix.
func TenantPrefix() string {
	return tenantPrefix
}
