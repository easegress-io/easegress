package registrycenter

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
)

// ServiceRegistryInfo contains service's spec,
//  and its instance, which is the sidecar+egress port address
type ServiceRegistryInfo struct {
	Service *spec.Service
	Ins     *spec.ServiceInstanceSpec   // indicates local egress
	RealIns []*spec.ServiceInstanceSpec // trully instance list in mesh
}

// UniqInstanceID creates a virutal uniq ID for every visible
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

func (rcs *Server) getTenants(tenantNames []string) map[string]*spec.Tenant {
	var tenants map[string]*spec.Tenant = make(map[string]*spec.Tenant)

	for _, v := range tenantNames {
		tenants[v] = rcs.service.GetTenantSpec(v)
	}

	return tenants
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
	if rcs.registered == false {
		return serviceInfo, spec.ErrNoRegisteredYet
	}

	tenants := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	service := rcs.service.GetServiceSpec(serviceName)
	if service == nil {
		return nil, spec.ErrServiceNotFound
	}

	var inGlobal bool = false
	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].Services {
			if v == serviceName {
				inGlobal = true
				break
			}
		}
	}

	if tenants[rcs.tenant] == nil {
		err := fmt.Errorf("BUG: can't find service:%s's registry tenant:%s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfo, err
	}

	if !inGlobal && service.RegisterTenant != rcs.tenant {
		return nil, spec.ErrServiceNotFound
	}

	return &ServiceRegistryInfo{
		Service: service,
		Ins:     rcs.defaultInstance(service),
	}, nil
}

// Discovery gets all services' spec and default instance(local sidecar for ever)
// which are visable for local service
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
	if rcs.registered == false {
		return serviceInfos, spec.ErrNoRegisteredYet
	}

	tenants := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].Services {
			if v != rcs.serviceName {
				visibleServices = append(visibleServices, v)
			}
		}
	}

	if tenants[rcs.tenant] == nil {
		err = fmt.Errorf("BUG: can't find service:%s's registry tenant:%s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfos, err
	}

	for _, v := range tenants[rcs.tenant].Services {
		visibleServices = append(visibleServices, v)
	}

	for _, v := range visibleServices {
		if service := rcs.service.GetServiceSpec(v); service == nil {
			logger.Errorf("can't find service :%s, failed , err:%v", v, err)
			continue
		} else {
			serviceInfos = append(serviceInfos, &ServiceRegistryInfo{
				Service: service,
				Ins:     rcs.defaultInstance(service),
			})
		}
	}

	return serviceInfos, err
}
