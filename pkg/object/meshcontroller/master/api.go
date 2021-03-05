package master

import (
	"fmt"
	"io/ioutil"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registry"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"
)

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

	var ins *registry.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return spec.ErrParamNotMatch
	}

	if err = m.mss.UpdateServiceInstanceLeases(ins.ServiceName, ins.InstanceID, ins.Leases); err != nil {
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

	var ins *registry.ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return spec.ErrParamNotMatch
	}

	if err = m.mss.UpdateServiceInstanceStatus(ins.ServiceName, ins.InstanceID, ins.Status); err != nil {
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

	if err := m.mss.DeleteSerivceInstance(serviceName, ID); err != nil {
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

	tenant, err := m.mss.GetTenant(tenantName)
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

	insList, err := m.mss.GetSerivceInstances(serviceName)
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
