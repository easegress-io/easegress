package master

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
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
)

func (m *Master) registerAPIs() {
	meshAPIs := []*api.APIEntry{
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
	serviceBuff, err := m.store.Get(layout.ServiceKey(serviceName))
	if err != nil {
		api.ClusterPanic(err)
	}
	if serviceBuff == nil {
		return nil
	}

	serviceSpec := &spec.Service{}
	yaml.Unmarshal([]byte(*serviceBuff), serviceSpec)

	if err != nil {
		panic(fmt.Errorf("unmarshal %s to service failed: %v", *serviceBuff, err))
	}

	return serviceSpec
}

func (m *Master) _putServiceSpec(serviceSpec *spec.Service) {
	serviceBuff, err := yaml.Marshal(serviceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", serviceSpec, err))
	}

	err = m.store.Put(layout.ServiceKey(serviceSpec.Name), string(serviceBuff))
	if err != nil {
		api.ClusterPanic(err)
	}
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

// UpdateSerivceInstanceLeases updates one serivce registry reord's lease
func (m *Master) UpdateSerivceInstanceLeases(ctx iris.Context) error {
	serviceName := ctx.Params().Get("service_name")
	ID := ctx.Params().Get("instance_id")

	if len(serviceName) == 0 || len(ID) == 0 {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("invalidate input , serivceName :%s, ID :%s ", serviceName, ID)
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return fmt.Errorf("read body failed: %v", err)
	}

	var ins *spec.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return spec.ErrParamNotMatch
	}

	if err = m.service.UpdateServiceInstanceLeases(ins.ServiceName, ins.InstanceID, ins.Leases); err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}
	return nil
}

// UpdateSerivceInstanceStatus updates one serivce registry reord's status
func (m *Master) UpdateSerivceInstanceStatus(ctx iris.Context) error {
	serviceName := ctx.Params().Get("service_name")
	ID := ctx.Params().Get("instance_id")

	if len(serviceName) == 0 || len(ID) == 0 {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("invalidate input , serivce name :%s, ID :%s ", serviceName, ID)
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return fmt.Errorf("read body failed: %v", err)
	}

	var ins *spec.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return spec.ErrParamNotMatch
	}

	if err = m.service.UpdateServiceInstanceStatus(ins.ServiceName, ins.InstanceID, ins.Status); err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}

	return nil
}

// DeleteServiceInstance deletes one service registry instance
func (m *Master) DeleteServiceInstance(ctx iris.Context) error {
	serviceName := ctx.Params().Get("service_name")
	ID := ctx.Params().Get("instance_id")

	if len(serviceName) == 0 || len(ID) == 0 {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("invalidate input , serivce name :%s, ID :%s ", serviceName, ID)
	}

	if err := m.service.DeleteSerivceInstance(serviceName, ID); err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)

		return err
	}

	return nil
}

// GetTenant gets services name list and its metadata for one specified tenant
func (m *Master) GetTenant(ctx iris.Context) error {
	tenantName := ctx.Params().Get("tenant_name")
	if len(tenantName) == 0 {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("invalidate input , tenant name:%s", tenantName)
	}

	tenant, err := m.service.GetTenant(tenantName)
	buff, err := yaml.Marshal(tenant)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		err = fmt.Errorf("marshal %#v to yaml failed: %v", tenant, err)
		logger.Errorf("BUG %v", err)
		return err
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
	return nil
}

// GetSerivceInstanceList returns services instance list.
func (m *Master) GetSerivceInstanceList(ctx iris.Context) error {
	serviceName := ctx.Params().Get("service_name")

	if len(serviceName) == 0 {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("invalidate input , serivce name :%s ", serviceName)
	}

	insList, err := m.service.GetSerivceInstances(serviceName)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}

	buff, err := yaml.Marshal(insList)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		err = fmt.Errorf("marshal %#v to yaml failed: %v", insList, err)
		logger.Errorf("BUG %v", err)
		return err
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)

	return nil
}
