package registrycenter

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type (
	// ServiceRegistryInfo contains service's spec,
	//  and its instance, which is the sidecar+egress port address
	ServiceRegistryInfo struct {
		Service *spec.Service
		Ins     *spec.ServiceInstanceSpec // indicates local egress
		Version int64                     // tenant ETCD key version,
	}

	tenantInfo struct {
		tenant *spec.Tenant
		info   *mvccpb.KeyValue
	}
)

// UniqInstanceID creates a virtual uniq ID for every visible
// service in mesh
func UniqInstanceID(serviceName string) string {
	return fmt.Sprintf("ins-%s-01", serviceName)
}

// GetServiceName split instanceID by '-' then return second
// field as the service name
func GetServiceName(instanceID string) string {
	names := strings.Split(instanceID, "-")

	if len(names) != 3 {
		return ""
	}
	return names[2]
}

// defaultInstance creates default egress instance point to the sidecar's egress port
func (rcs *Server) defaultInstance(service *spec.Service) *spec.ServiceInstanceSpec {
	return &spec.ServiceInstanceSpec{
		ServiceName: service.Name,
		InstanceID:  UniqInstanceID(service.Name),
		IP:          service.Sidecar.Address,
		Port:        uint32(service.Sidecar.EgressPort),
	}
}

func (rcs *Server) getTenants(tenantNames []string) map[string]*tenantInfo {
	tenantInfos := make(map[string]*tenantInfo)

	for _, v := range tenantNames {
		tenant, info := rcs.service.GetTenantSpecWithInfo(v)
		if tenant != nil {
			tenantInfos[v] = &tenantInfo{
				tenant: tenant,
				info:   info,
			}
		}
	}

	return tenantInfos
}

// DiscoveryService gets one service specs with default instance
func (rcs *Server) DiscoveryService(serviceName string) (*ServiceRegistryInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("registry center recover from: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()
	var serviceInfo *ServiceRegistryInfo
	if !rcs.registered {
		return serviceInfo, spec.ErrNoRegisteredYet
	}

	tenants := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	service := rcs.service.GetServiceSpec(serviceName)
	if service == nil {
		return nil, spec.ErrServiceNotFound
	}

	var inGlobal bool = false
	if globalTenant, ok := tenants[spec.GlobalTenant]; ok {
		for _, v := range globalTenant.tenant.Services {
			if v == serviceName {
				inGlobal = true
				break
			}
		}
	}

	if _, ok := tenants[rcs.tenant]; !ok {
		err := fmt.Errorf("BUG: can't find service: %s's registry tenant: %s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfo, err
	}

	if !inGlobal && service.RegisterTenant != rcs.tenant {
		return nil, spec.ErrServiceNotFound
	}

	return &ServiceRegistryInfo{
		Service: service,
		Ins:     rcs.defaultInstance(service),
		Version: tenants[rcs.tenant].info.Version,
	}, nil
}

// Discovery gets all services' spec and default instance(local sidecar for ever)
// which are visible for local service
func (rcs *Server) Discovery() ([]*ServiceRegistryInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("registry center recover from: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()
	var (
		serviceInfos    []*ServiceRegistryInfo
		visibleServices []string
		err             error
	)
	if !rcs.registered {
		return serviceInfos, spec.ErrNoRegisteredYet
	}
	var version int64
	tenantInfos := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if globalTentant, ok := tenantInfos[spec.GlobalTenant]; ok {
		version = globalTentant.info.Version
		for _, v := range tenantInfos[spec.GlobalTenant].tenant.Services {
			if v != rcs.serviceName {
				visibleServices = append(visibleServices, v)
			}
		}
	}

	if tenant, ok := tenantInfos[rcs.tenant]; !ok {
		err = fmt.Errorf("BUG: can't find service: %s's registry tenant: %s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfos, err
	} else {
		if tenant.info.Version > version {
			version = tenant.info.Version
		}
	}

	visibleServices = append(visibleServices, tenantInfos[rcs.tenant].tenant.Services...)

	for _, v := range visibleServices {
		if service := rcs.service.GetServiceSpec(v); service == nil {
			logger.Errorf("can't find service: %s failed: %v", v, err)
			continue
		} else {
			serviceInfos = append(serviceInfos, &ServiceRegistryInfo{
				Service: service,
				Ins:     rcs.defaultInstance(service),
				Version: version,
			})
		}
	}

	return serviceInfos, err
}
