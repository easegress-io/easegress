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
	specs := m.service.listServiceSpecs()

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
	}

	m.storageLock()
	defer m.storageUnlock()

	oldSpec := m.service.getServiceSpec(serviceName)
	if oldSpec != nil {
		api.HandleAPIError(ctx, http.StatusConflict, fmt.Errorf("%s existed", serviceName))
		return
	}

	m.service.putServiceSpec(serviceSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) getService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := m.service.getServiceSpec(serviceName)
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
	}

	m.storageLock()
	defer m.storageUnlock()

	oldSpec := m.service.getServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	m.service.putServiceSpec(serviceSpec)
}

func (m *Master) deleteService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.storageLock()
	defer m.storageUnlock()

	oldSpec := m.service.getServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	m.service.deleteServiceSpec(serviceName)
}
