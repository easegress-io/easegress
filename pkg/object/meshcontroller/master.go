package meshcontroller

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/logger"
)

// Master is the role of EG for mesh control plane
type Master struct {
	mss *MeshServiceServer

	ServiceWatchInterval string

	done chan struct{}
}

// NewMaster return a ini
func NewMaster() *Master {

	return &Master{}
}

func (m *Master) watchServicesHeartbeat() {

	// Get all serivces
	m.mss.WatchSerivceInstancesHeartbeat()
	// find one serivce instance

	// read heartbeat, if more than 30s (configurable), then set the instance to OUT_OF_SERVICE

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
		return fmt.Errorf("invalidate input , serivceName :%s, ID :%s ", serviceName, ID)
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	var ins *ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		return ErrParamNotMatch
	}

	return m.mss.UpdateServiceInstanceLeases(ins.ServiceName, ins.InstanceID, ins.Leases)

}

// UpdateSerivceInstanceStatus updates one serivce registry reord's status
func (m *Master) UpdateSerivceInstanceStatus(ctx iris.Context) error {

	serviceName := ctx.Params().Get("service_name")
	ID := ctx.Params().Get("instance_id")

	if len(serviceName) == 0 || len(ID) == 0 {
		return fmt.Errorf("invalidate input , serivceName :%s, ID :%s ", serviceName, ID)
	}

	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	var ins *ServiceInstance
	if err := yaml.Unmarshal(body, &ins); err != nil {
		return fmt.Errorf("unmarshal service: %s's instance body failed, err %s ", serviceName, err)
	}

	if ins.ServiceName != serviceName || ins.InstanceID != ID {
		return ErrParamNotMatch
	}

	return m.mss.UpdateServiceInstanceStatus(ins.ServiceName, ins.InstanceID, ins.Status)

}

// DeleteServiceInstance deletes one service registry instance
func (m *Master) DeleteServiceInstance(ctx iris.Context) error {
	serviceName := ctx.Params().Get("service_name")
	ID := ctx.Params().Get("instance_id")

	if len(serviceName) == 0 || len(ID) == 0 {
		return fmt.Errorf("invalidate input , serivceName :%s, ID :%s ", serviceName, ID)
	}

	return m.mss.DeleteSerivceInstance(serviceName, ID)
}
