package registrycenter

import (
	"fmt"
	"strings"

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
	Ins     *spec.ServiceInstance   // indicates local egress
	RealIns []*spec.ServiceInstance // trully instance list in mesh
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
func (rcs *Server) defaultInstance(service *spec.Service) *spec.ServiceInstance {
	return &spec.ServiceInstance{
		ServiceName: service.Name,
		InstanceID:  UniqInstanceID(service.Name),
		IP:          service.Sidecar.Address,
		Port:        uint32(service.Sidecar.EgressPort),
	}
}

func (rcs *Server) discoverySelf(service *spec.Service) *spec.ServiceInstance {
	return &spec.ServiceInstance{
		ServiceName: rcs.serviceName,
		InstanceID:  UniqInstanceID(rcs.serviceName),
		IP:          service.Sidecar.Address,
		Port:        uint32(service.Sidecar.IngressPort),
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

// DiscoveryService gets one service specs with default instance
func (rcs *Server) DiscoveryService(serviceName string) (*ServiceRegistryInfo, error) {
	var serviceInfo *ServiceRegistryInfo
	if rcs.registried == false {
		return serviceInfo, ErrNoRegistriedYet
	}

	tenants, err := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if err != nil {
		return serviceInfo, err
	}

	service, err := rcs.getService(serviceName)
	if err != nil {
		return nil, err
	}
	// discovery itself
	if serviceName == rcs.serviceName {

		return &ServiceRegistryInfo{
			Service: service,
			Ins:     rcs.discoverySelf(service),
		}, nil
	}

	var inGlobal bool = false
	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].ServicesList {
			if v == serviceName {
				inGlobal = true
				break
			}
		}
	}

	if tenants[rcs.tenant] == nil {
		err = fmt.Errorf("service %s, registered to unknow tenant %s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfo, err
	}

	if !inGlobal && service.RegisterTenant != rcs.tenant {
		return nil, ErrServiceNotFound
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
	if rcs.registried == false {
		return serviceInfos, ErrNoRegistriedYet
	}

	tenants, err := rcs.getTenants([]string{spec.GlobalTenant, rcs.tenant})
	if err != nil {
		return serviceInfos, err
	}

	if tenants[spec.GlobalTenant] != nil {
		for _, v := range tenants[spec.GlobalTenant].ServicesList {
			if v != rcs.serviceName {
				visibleServices = append(visibleServices, v)
			}
		}
	}

	if tenants[rcs.tenant] == nil {
		err = fmt.Errorf("service %s, registered to unknow tenant %s", rcs.serviceName, rcs.tenant)
		logger.Errorf("%v", err)
		return serviceInfos, err
	} else {
		for _, v := range tenants[rcs.tenant].ServicesList {
			if v != rcs.serviceName {
				visibleServices = append(visibleServices, v)
			}
		}
	}

	for _, v := range visibleServices {
		if service, err := rcs.getService(v); err != nil {
			logger.Errorf("worker:%s get service :%s, failed , err:%v", rcs.serviceName, v, err)
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
