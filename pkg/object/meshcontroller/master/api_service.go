package master

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/stringtool"

	"gopkg.in/yaml.v2"
)

type servicesByOrder []*spec.Service

func (s servicesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s servicesByOrder) Len() int           { return len(s) }
func (s servicesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readServiceName(ctx iris.Context) (string, error) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

func (m *Master) listServices(ctx iris.Context) {
	specs := m.service.ListServiceSpecs()

	sort.Sort(servicesByOrder(specs))

	buff, err := yaml.Marshal(specs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) createService(ctx iris.Context) {
	serviceSpec := &spec.Service{}

	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec != nil {
		api.HandleAPIError(ctx, http.StatusConflict, fmt.Errorf("%s existed", serviceName))
		return
	}

	tenantSpec := m.service.GetTenantSpec(serviceSpec.RegisterTenant)
	if tenantSpec == nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
		return
	}

	tenantSpec.Services = append(tenantSpec.Services, serviceSpec.RegisterTenant)

	m.service.PutServiceSpec(serviceSpec)
	m.service.PutTenantSpec(tenantSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) getService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := m.service.GetServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	buff, err := yaml.Marshal(serviceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", serviceSpec, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) updateService(ctx iris.Context) {
	serviceSpec := &spec.Service{}

	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	if serviceSpec.RegisterTenant != oldSpec.RegisterTenant {
		newTenantSpec := m.service.GetTenantSpec(serviceSpec.RegisterTenant)
		if newTenantSpec == nil {
			api.HandleAPIError(ctx, http.StatusBadRequest,
				fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
			return
		}
		newTenantSpec.Services = append(newTenantSpec.Services, serviceSpec.RegisterTenant)

		oldTenantSpec := m.service.GetTenantSpec(oldSpec.RegisterTenant)
		if oldTenantSpec == nil {
			panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
		}
		oldTenantSpec.Services = stringtool.DeleteStrInSlice(oldTenantSpec.Services, serviceName)

		m.service.PutTenantSpec(newTenantSpec)
		m.service.PutTenantSpec(oldTenantSpec)
	}

	m.service.PutServiceSpec(serviceSpec)
}

func (m *Master) deleteService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	tenantSpec := m.service.GetTenantSpec(oldSpec.RegisterTenant)
	if tenantSpec == nil {
		panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
	}

	tenantSpec.Services = stringtool.DeleteStrInSlice(tenantSpec.Services, serviceName)

	m.service.PutTenantSpec(tenantSpec)
	m.service.DeleteServiceSpec(serviceName)
}
