package master

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"

	"gopkg.in/yaml.v2"
)

type tenantsByOrder []*spec.Tenant

func (s tenantsByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s tenantsByOrder) Len() int           { return len(s) }
func (s tenantsByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *Master) readTenantName(ctx iris.Context) (string, error) {
	serviceName := ctx.Params().Get("tenantName")
	if serviceName == "" {
		return "", fmt.Errorf("empty tenant name")
	}

	return serviceName, nil
}

func (m *Master) listTenants(ctx iris.Context) {
	specs := m.service.ListTenantSpecs()

	sort.Sort(tenantsByOrder(specs))

	buff, err := yaml.Marshal(specs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) createTenant(ctx iris.Context) {
	tenantSpec := &spec.Tenant{}

	tenantName, err := m.readTenantName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, tenantSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if tenantName != tenantSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", tenantName, tenantSpec.Name))
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec != nil {
		api.HandleAPIError(ctx, http.StatusConflict, fmt.Errorf("%s existed", tenantName))
		return
	}

	m.service.PutTenantSpec(tenantSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) getTenant(ctx iris.Context) {
	tenantName, err := m.readTenantName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	tenantSpec := m.service.GetTenantSpec(tenantName)
	if tenantSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	buff, err := yaml.Marshal(tenantSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", tenantSpec, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) updateTenant(ctx iris.Context) {
	tenantSpec := &spec.Tenant{}

	tenantName, err := m.readTenantName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	err = m.readSpec(ctx, tenantSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	if tenantName != tenantSpec.Name {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", tenantName, tenantSpec.Name))
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}

	if tenantSpec.CreatedAt == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("createdAt are not empty"))
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	// NOTE: The fields below can't be updated.
	tenantSpec.Services, tenantSpec.CreatedAt = oldSpec.Services, oldSpec.CreatedAt

	m.service.PutTenantSpec(tenantSpec)
}

func (m *Master) deleteTenant(ctx iris.Context) {
	tenantName, err := m.readTenantName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	oldSpec := m.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	if len(oldSpec.Services) != 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("%s got services: %v", tenantName, oldSpec.Services))
		return
	}

	m.service.DeleteTenantSpec(tenantName)
}
