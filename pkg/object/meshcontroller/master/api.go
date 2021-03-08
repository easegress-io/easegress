package master

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/v"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshServicePrefix is the mesh service prefix
	MeshServicePrefix = MeshPrefix + "/services/{serviceName:string}"

	// MeshRegistryPrefix is the mesh service registry prefix
	MeshRegistryPrefix = MeshPrefix + "/registry/{serviceName:string}/{instanceID:string}"

	// MeshTenantPrefix is the mesh tenant prefix
	MeshTenantPrefix = MeshPrefix + "/tenant/{tenantName:string}"
)

func (m *Master) registerAPIs() {
	meshAPIs := []*api.APIEntry{
		{
			Path:    MeshServicePrefix,
			Method:  "GET",
			Handler: m.getServices,
		},
		{
			Path:    MeshServicePrefix,
			Method:  "POST",
			Handler: m.createService,
		},
		{
			Path:    MeshServicePrefix,
			Method:  "PUT",
			Handler: m.updateService,
		},
		{
			Path:    MeshServicePrefix,
			Method:  "DELETE",
			Handler: m.deleteService,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "GET",
			Handler: m.getCanary,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "POST",
			Handler: m.createCanary,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "PUT",
			Handler: m.updateCanary,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "DELETE",
			Handler: m.deleteCanary,
		},
		{
			Path:    MeshServicePrefix + "/loadbalance",
			Method:  "GET",
			Handler: m.getLoadBalance,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "POST",
			Handler: m.createLoadBalance,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "PUT",
			Handler: m.updateLoadBalance,
		},
		{
			Path:    MeshServicePrefix + "/canary",
			Method:  "DELETE",
			Handler: m.deleteLoadBalance,
		},
		{
			Path:    MeshRegistryPrefix,
			Method:  "DELETE",
			Handler: m.deleteServiceInstance,
		},
		{
			Path:    MeshRegistryPrefix,
			Method:  "GET",
			Handler: m.getSerivceInstanceList,
		},
		{
			Path:    MeshRegistryPrefix,
			Method:  "PUT",
			Handler: m.updateSerivceInstanceLeases,
		},
		{
			Path:    MeshRegistryPrefix,
			Method:  "PUT",
			Handler: m.updateSerivceInstanceStatus,
		},
		{
			Path:    MeshTenantPrefix,
			Method:  "GET",
			Handler: m.getTenant,
		},
	}

	api.GlobalServer.RegisterAPIs(meshAPIs)
}

func (m *Master) storageLock() {
	err := m.store.Lock()
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (m *Master) storageUnlock() {
	err := m.store.Unlock()
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (m *Master) readServiceName(ctx iris.Context) (string, error) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

func (m *Master) readSpec(ctx iris.Context, spec interface{}) (string, interface{}, error) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		return "", nil, err
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return "", nil, fmt.Errorf("read body failed: %v", err)
	}

	err = yaml.Unmarshal(body, spec)
	if err != nil {
		return "", nil, fmt.Errorf("unmarshal %#v to yaml: %v", spec, err)
	}

	vr := v.Validate(spec, body)
	if !vr.Valid() {
		return "", nil, fmt.Errorf("validate failed: \n%s", vr)
	}

	return serviceName, spec, err
}

func (m *Master) _getServiceSpec(serviceName string) *spec.Service {
	service, err := m.service.GetService(serviceName)
	if err != nil {
		api.ClusterPanic(err)
	}
	return service
}

func (m *Master) _putServiceSpec(serviceSpec *spec.Service) {
	err := m.service.UpdateServiceSpec(serviceSpec)
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (m *Master) getServices(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	svcList, err := m.service.GetServiceList(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	buff, err := yaml.Marshal(svcList)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("marshal %#v to yaml failed: %v", svcList, err))
		return
	}
	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)

}

func (m *Master) createService(ctx iris.Context) {
	serviceSpec := &spec.Service{}

	serviceName, _spec, err := m.readSpec(ctx, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	_, ok := _spec.(*spec.Service)
	if !ok {
		panic(fmt.Errorf("want *spec.Service, got %T", _spec))
	}

	oldService := m._getServiceSpec(serviceName)
	if oldService != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("service %s already exists", serviceName))
		return
	}

	m.storageLock()
	defer m.storageUnlock()

	err = m.service.CreateService(serviceSpec)
	if err != nil {
		api.ClusterPanic(err)
	}

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) updateService(ctx iris.Context) {

	serviceSpec := &spec.Service{}
	_, _spec, err := m.readSpec(ctx, serviceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	_, ok := _spec.(*spec.Service)
	if !ok {
		panic(fmt.Errorf("want *spec.Service, got %T", _spec))
	}

	m.storageLock()
	defer m.storageUnlock()

	m._putServiceSpec(serviceSpec)
	ctx.StatusCode(http.StatusOK)
}

func (m *Master) deleteService(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.storageLock()
	defer m.storageUnlock()

	err = m.service.DeleteService(serviceName)
	if err != nil {
		api.ClusterPanic(err)
	}
	ctx.StatusCode(http.StatusOK)

}

func (m *Master) getCanary(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	// NOTE: No need to lock.
	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.Canary == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("canary of service %s not found", serviceName))
		return
	}

	buff, err := yaml.Marshal(serviceSpec.Canary)
	if err != nil {
		panic(err)
	}
	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) createCanary(ctx iris.Context) {
	canarySpec := &spec.Canary{}

	serviceName, _spec, err := m.readSpec(ctx, canarySpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	canary, ok := _spec.(*spec.Canary)
	if !ok {
		panic(fmt.Errorf("want *spec.Canary, got %T", _spec))
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.Canary != nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("canary of service %s already exited", serviceName))
		return
	}

	serviceSpec.Canary = canary
	m._putServiceSpec(serviceSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) updateCanary(ctx iris.Context) {
	canarySpec := &spec.Canary{}

	serviceName, _spec, err := m.readSpec(ctx, canarySpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	canary, ok := _spec.(*spec.Canary)
	if !ok {
		panic(fmt.Errorf("want *spec.Canary, got %T", _spec))
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.Canary == nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("canary of service %s not found", serviceName))
		return
	}

	serviceSpec.Canary = canary
	m._putServiceSpec(serviceSpec)

	ctx.StatusCode(http.StatusOK)
}

func (m *Master) deleteCanary(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.Canary == nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("canary of service %s not found", serviceName))
		return
	}

	serviceSpec.Canary = nil
	m._putServiceSpec(serviceSpec)

	ctx.StatusCode(http.StatusOK)
}

func (m *Master) getLoadBalance(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	// NOTE: No need to lock.
	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.LoadBalance == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("loadBalance of service %s not found", serviceName))
		return
	}

	buff, err := yaml.Marshal(serviceSpec.LoadBalance)
	if err != nil {
		panic(err)
	}
	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
}

func (m *Master) createLoadBalance(ctx iris.Context) {
	loadBalanceSpec := &spec.LoadBalance{}

	serviceName, _spec, err := m.readSpec(ctx, loadBalanceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	loadBalance, ok := _spec.(*spec.LoadBalance)
	if !ok {
		panic(fmt.Errorf("want *spec.LoadBalance, got %T", _spec))
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.LoadBalance != nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("loadBalance of service %s already exited", serviceName))
		return
	}

	serviceSpec.LoadBalance = loadBalance
	m._putServiceSpec(serviceSpec)

	ctx.Header("Location", ctx.Path())
	ctx.StatusCode(http.StatusCreated)
}

func (m *Master) updateLoadBalance(ctx iris.Context) {
	loadBalanceSpec := &spec.LoadBalance{}

	serviceName, _spec, err := m.readSpec(ctx, loadBalanceSpec)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}
	loadBalance, ok := _spec.(*spec.LoadBalance)
	if !ok {
		panic(fmt.Errorf("want *spec.LoadBalance, got %T", _spec))
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.LoadBalance == nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("loadBalance of service %s not found", serviceName))
		return
	}

	serviceSpec.LoadBalance = loadBalance
	m._putServiceSpec(serviceSpec)

	ctx.StatusCode(http.StatusOK)
}

func (m *Master) deleteLoadBalance(ctx iris.Context) {
	serviceName, err := m.readServiceName(ctx)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	m.storageLock()
	defer m.storageUnlock()

	serviceSpec := m._getServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(ctx, http.StatusNotFound,
			fmt.Errorf("service %s not found", serviceName))
		return
	}

	if serviceSpec.LoadBalance == nil {
		api.HandleAPIError(ctx, http.StatusConflict,
			fmt.Errorf("loadBalance of service %s not found", serviceName))
		return
	}

	serviceSpec.LoadBalance = nil
	m._putServiceSpec(serviceSpec)

	ctx.StatusCode(http.StatusOK)
}

// updateSerivceInstanceLeases updates one serivce registry reord's lease
func (m *Master) updateSerivceInstanceLeases(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	ID := ctx.Params().Get("instanceID")

	if len(serviceName) == 0 || len(ID) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("invalidate input , serivce name :%s, ID :%s ", serviceName, ID))

		return
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("read body failed: %v", err))
		return
	}

	var ins *spec.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err))
		return
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("url service name %s, instatnce id %s, yaml service name %s, instance id %s not matched",
				ins.ServiceName, serviceName, ins.InstanceID, ID))
		return
	}

	if err = m.service.UpdateServiceInstanceLeases(ins.ServiceName, ins.InstanceID, ins.Leases); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	return
}

// updateSerivceInstanceStatus updates one serivce registry reord's status
func (m *Master) updateSerivceInstanceStatus(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	ID := ctx.Params().Get("instanceID")

	if len(serviceName) == 0 || len(ID) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("invalidate input , serivce name :%s, ID :%s ", serviceName, ID))
		return
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("read body failed: %v", err))
		return
	}

	var ins *spec.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err))
		return
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		api.HandleAPIError(ctx, http.StatusBadRequest, spec.ErrParamNotMatch)
		return
	}

	if err = m.service.UpdateServiceInstanceStatus(ins.ServiceName, ins.InstanceID, ins.Status); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	return
}

// deleteServiceInstance deletes one service registry instance
func (m *Master) deleteServiceInstance(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	ID := ctx.Params().Get("instanceID")

	if len(serviceName) == 0 || len(ID) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("invalidate input , serivce name :%s, ID :%s ", serviceName, ID))
		return
	}

	if err := m.service.DeleteSerivceInstance(serviceName, ID); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)

		return
	}

	return
}

// getTenant gets services name list and its metadata for one specified tenant
func (m *Master) getTenant(ctx iris.Context) {
	tenantName := ctx.Params().Get("tenantName")
	if len(tenantName) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("invalidate input , tenant name:%s", tenantName))
		return
	}

	tenant, err := m.service.GetTenant(tenantName)
	buff, err := yaml.Marshal(tenant)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("marshal %#v to yaml failed: %v", tenant, err))
		return
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
	return
}

// getSerivceInstanceList returns services instance list.
func (m *Master) getSerivceInstanceList(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")

	if len(serviceName) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("invalidate input , serivce name :%s ", serviceName))
		return
	}

	insList, err := m.service.GetSerivceInstances(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	buff, err := yaml.Marshal(insList)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError,
			fmt.Errorf("marshal %#v to yaml failed: %v", insList, err))
		return
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)

	return
}
