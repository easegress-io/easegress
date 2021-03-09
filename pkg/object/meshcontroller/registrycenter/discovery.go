package registrycenter

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
)

var serviceNoFoundFormat = "can't find %s service in tenant :%s and global"

// ServiceRegistryInfo contains service spec,
//  and its instance lists
type ServiceRegistryInfo struct {
	Service *spec.Service
	Ins     *spec.ServiceInstance
}

// defaultInstance creates default egress instance point to the sidecar's egress port
func (rcs *Server) defaultInstance(serviceName string, service *spec.Service) *spec.ServiceInstance {
	return &spec.ServiceInstance{
		ServiceName: serviceName,
		InstanceID:  rcs.instanceID,
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
	serviceSpec, err := rcs.store.Get(layout.ServiceKey(rcs.serviceName))
	if err != nil {
		logger.Errorf("Get %s ServiceSpec failed, err :%v", rcs.serviceName, err)
		return nil, err
	}

	err = yaml.Unmarshal([]byte(*serviceSpec), service)
	if err != nil {
		logger.Errorf("BUG, unmarshal Service : %s,failed, err : %v", rcs.serviceName, err)
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
		tenantSpec, err := rcs.store.Get(layout.TenantKey(v))
		if err != nil {
			logger.Errorf("Get %s ServiceSpec failed, err :%v", rcs.serviceName, err)
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

func (rcs *Server) getBasicInfo() (map[string]*spec.Tenant, *spec.Service, error) {
	var (
		err     error
		tenants map[string]*spec.Tenant = make(map[string]*spec.Tenant)
		service *spec.Service
	)

	if rcs.registried == false {
		logger.Errorf("registry center receive discovery req before it registries successful")
		return tenants, service, err
	}

	if service, err = rcs.getService(rcs.serviceName); err != nil {
		return tenants, service, err
	}

	if tenants, err = rcs.getTenants([]string{spec.GlobalTenant, service.RegisterTenant}); err != nil {
		logger.Errorf("get tenants: %vfailed, err:%v ", []string{spec.GlobalTenant, service.RegisterTenant}, err)
		return tenants, service, err
	}

	return tenants, service, nil
}

// DiscoveryService gets one service specs with default instance
func (rcs *Server) DiscoveryService(serviceName string) (*ServiceRegistryInfo, error) {
	var serviceInfo *ServiceRegistryInfo

	if rcs.registried == false {
		return serviceInfo, ErrNoRegistriedYet
	}

	tenants, service, err := rcs.getBasicInfo()
	if err != nil {
		return serviceInfo, err
	}

	// discovery itself
	if serviceName == rcs.serviceName {
		return &ServiceRegistryInfo{
			Service: service,
			Ins:     rcs.defaultInstance(rcs.serviceName, service),
		}, nil
	}

	var (
		inGlobal      bool = false
		inSameTenant  bool = false
		targetService *spec.Service
	)

	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].ServicesList {
			if v == serviceName {
				inGlobal = true
				break
			}
		}
	}

	if tenants[service.RegisterTenant] == nil {
		err = fmt.Errorf("service %s, registered to unknow tenant %s", rcs.serviceName, service.RegisterTenant)
		logger.Errorf("%v", err)
		return serviceInfo, err

	}
	if !inGlobal {
		for _, v := range tenants[service.RegisterTenant].ServicesList {
			if v == serviceName {
				inSameTenant = true
				break
			}
		}
	}

	if !inGlobal && !inSameTenant {
		return nil, ErrServiceNotFound
	}
	if targetService, err = rcs.getService(serviceName); err != nil {
		return nil, err
	}

	return &ServiceRegistryInfo{
		Service: targetService,
		Ins:     rcs.defaultInstance(serviceName, service),
	}, nil
}

// Discovery gets all services' spec and default instance(local sidecar for ever)
// which are visable for local service
func (rcs *Server) Discovery() ([]*ServiceRegistryInfo, error) {
	var (
		serviceInfos    []*ServiceRegistryInfo
		visableServices []string
	)

	tenants, service, err := rcs.getBasicInfo()
	if err != nil {
		return serviceInfos, err
	}

	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].ServicesList {
			if v != rcs.serviceName {
				visableServices = append(visableServices, v)
			}
		}
	}

	if tenants[service.RegisterTenant] == nil {
		err = fmt.Errorf("service %s, registered to unknow tenant %s", rcs.serviceName, service.RegisterTenant)
		logger.Errorf("%v", err)
		return serviceInfos, err
	} else {
		for _, v := range tenants[service.RegisterTenant].ServicesList {
			if v != rcs.serviceName {
				visableServices = append(visableServices, v)
			}
		}
	}

	for _, v := range visableServices {
		if service, err := rcs.getService(v); err != nil {
			logger.Errorf("worker:%s get service :%s, failed , err:%v", rcs.serviceName, v, err)
			continue
		} else {
			serviceInfos = append(serviceInfos, &ServiceRegistryInfo{
				Service: service,
				Ins:     rcs.defaultInstance(service.Name, service),
			})
		}
	}

	return serviceInfos, err
}
