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

// GenServerKey generates storage serivce key
func GenServerKey(serviceName string) string {
	return fmt.Sprintf(serviceSpecFormat, serviceName)
}

// GenServiceInstanceKey generates storage instance key
func GenServiceInstanceKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceFormat, serviceName, instanceID)
}

// GenServiceInstancePrefix generates one serivce's storage instance prefix
func GenServiceInstancePrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstancePrefixFormat, serviceName)
}

// GenServiceHeartbeatKey generates storage instance hearbeat key
func GenServiceHeartbeatKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceFormat, serviceName, instanceID)
}

// GenTenantKey generates storage tenant key
func GenTenantKey(tenant string) string {
	return fmt.Sprintf(tenantFormat, tenant)
}

// GetTenantPrefix gets the tenant storage prefix
func GetTenantPrefix() string {
	return tenantPrefix
}
