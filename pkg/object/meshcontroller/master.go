package meshcontroller

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/supervisor"
)

// Master is the role of EG for mesh control plane
type Master struct {
	super *supervisor.Supervisor
	mss   *MeshServiceServer

	ServiceWatchInterval string

	done chan struct{}
}

// NewMaster return a initialized master
func NewMaster(spec *Spec, super *supervisor.Supervisor) *Master {
	store := &mockEtcdClient{}
	serviceServer := NewDefaultMeshServiceServer(store, nil)

	return &Master{
		super:                super,
		mss:                  serviceServer,
		ServiceWatchInterval: spec.SpecUpdateInterval,
	}
}

func (m *Master) watchServicesHeartbeat() {
	m.mss.WatchSerivceInstancesHeartbeat()

	return
}

// Run is the entry of the Master role controller
func (m *Master) Run() {
	watchInterval, err := time.ParseDuration(m.ServiceWatchInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			m.ServiceWatchInterval, err)
		return
	}

	for {
		select {
		case <-m.done:
			return
		case <-time.After(watchInterval):
			m.watchServicesHeartbeat()
		}
	}
}

// Close closes the master
func (m *Master) Close() {
	m.done <- struct{}{}
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

	var ins *ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return ErrParamNotMatch
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

	var ins *ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		ctx.StatusCode(iris.StatusBadRequest)
		return ErrParamNotMatch
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

	tenant, err := m.mss.GetTenantSpec(tenantName)
	buff, err := yaml.Marshal(tenant)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		err = fmt.Errorf("marshal %#v to yaml failed: %v", tenant, err)
		logger.Errorf("[BUG] %v", err)
		return err
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)
	return nil
}

// GetServiceInstancesList
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
		logger.Errorf("[BUG] %v", err)
		return err
	}

	ctx.Header("Content-Type", "text/vnd.yaml")
	ctx.Write(buff)

	return nil
}
