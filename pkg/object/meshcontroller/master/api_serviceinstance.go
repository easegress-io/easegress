package master

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"

	"gopkg.in/yaml.v2"
)

type serviceInstancesByOrder []*spec.ServiceInstanceSpec

func (s serviceInstancesByOrder) Less(i, j int) bool {
	return s[i].ServiceName < s[j].ServiceName || s[i].InstanceID < s[j].InstanceID
}
func (s serviceInstancesByOrder) Len() int      { return len(s) }
func (s serviceInstancesByOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (m *Master) readServiceInstanceInfo(ctx iris.Context) (string, string, error) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		return "", "", fmt.Errorf("empty service name")
	}

	instanceID := ctx.Params().Get("instanceID")
	if instanceID == "" {
		return "", "", fmt.Errorf("empty instance id")
	}

	return serviceName, instanceID, nil
}

func (m *Master) listServiceInstanceSpecs(ctx iris.Context) {
	specs := m.service.ListAllServiceInstanceSpecs()

	sort.Sort(serviceInstancesByOrder(specs))

	buff, err := yaml.Marshal(specs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", specs, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) getServiceInstanceSpec(ctx iris.Context) {
	serviceName, instanceID, err := m.readServiceInstanceInfo(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := m.service.GetServiceInstanceSpec(serviceName, instanceID)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s/%s not found", serviceName, instanceID))
		return
	}

	buff, err := yaml.Marshal(serviceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", serviceSpec, err))
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) offlineSerivceInstance(ctx iris.Context) {
	serviceName, instanceID, err := m.readServiceInstanceInfo(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.service.Lock()
	defer m.service.Unlock()

	instanceSpec := m.service.GetServiceInstanceSpec(serviceName, instanceID)
	if instanceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound, fmt.Errorf("%s/%s not found", serviceName, instanceID))
		return
	}

	instanceSpec.Status = registrycenter.SerivceStatusOutOfSerivce
	m.service.PutServiceInstanceSpec(instanceSpec)
}
