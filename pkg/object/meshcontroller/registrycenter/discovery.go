package registrycenter

import (
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
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

func (rcs *Server) getService(serviceName string) (*spec.Service, error) {
	var (
		service *spec.Service
		err     error
	)
	// find which service inside the same tenant
	serviceSpec, err := rcs.store.Get(layout.ServiceSpecKey(rcs.serviceName))
	if err != nil {
		logger.Errorf("get service:%s failed, err :%v", rcs.serviceName, err)
		return nil, err
	}

	if len(*serviceSpec) == 0 {
		return nil, spec.ErrServiceNotFound
	}

	err = yaml.Unmarshal([]byte(*serviceSpec), service)
	if err != nil {
		logger.Errorf("BUG: unmarshal service : %s,failed, err : %v", rcs.serviceName, err)
		return nil, err
	}
	return service, nil
}

func (rcs *Server) getTenants(tenantNames []string) (map[string]*spec.Tenant, error) {
	var (
		tenants map[string]*spec.Tenant = make(map[string]*spec.Tenant)
		err     error
	)

	for _, v := range tenantNames {
		var tenant spec.Tenant
		tenantSpec, err := rcs.store.Get(layout.TenantSpecKey(v))
		if err != nil {
			logger.Errorf("get service:%s failed, err :%v", rcs.serviceName, err)
			return tenants, err
		}
		if len(*tenantSpec) == 0 {
			tenants[v] = nil
		}

		err = yaml.Unmarshal([]byte(*tenantSpec), &tenant)
		if err != nil {
			logger.Errorf("BUG, unmarshal tenant : %s,failed, err : %v", v, err)
			return tenants, err
		}
	}

	return tenants, err
}

// DiscoveryService gets one service specs with default instance
func (rcs *Server) DiscoveryService(serviceName string) (*ServiceRegistryInfo, error) {
	var serviceInfo *ServiceRegistryInfo
	if rcs.registered == false {
		return serviceInfo, spec.ErrNoRegisteredYet
	}

	tenants, err := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if err != nil {
		return serviceInfo, err
	}

	service, err := rcs.getService(serviceName)
	if err != nil {
		return nil, err
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
		err = fmt.Errorf("BUG: can't find service:%s's registry tenant:%s", rcs.serviceName, rcs.tenant)
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
	var (
		serviceInfos    []*ServiceRegistryInfo
		visibleServices []string
	)
	if rcs.registered == false {
		return serviceInfos, spec.ErrNoRegisteredYet
	}

	tenants, err := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if err != nil {
		return serviceInfos, err
	}

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
		if v != rcs.serviceName {
			visibleServices = append(visibleServices, v)
		}
	}

	for _, v := range visibleServices {
		if service, err := rcs.getService(v); err != nil {
			logger.Errorf("get service :%s, failed , err:%v", rcs.serviceName, err)
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
