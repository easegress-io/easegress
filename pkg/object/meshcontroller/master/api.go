package master

import (
	"fmt"
	"io/ioutil"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/v"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshTenantPrefix is the mesh tenant prefix.
	MeshTenantPrefix = "/mesh/tenants"

	// MeshTenantPath is the mesh tenant path.
	MeshTenantPath = "/mesh/tenants/{tenantName:string}"

	// MeshServicePrefix is mesh service prefix.
	MeshServicePrefix = "/mesh/services"

	// MeshServicePath is the mesh service path.
	MeshServicePath = "/mesh/services/{serviceName:string}"

	// MeshServiceCanaryPath is the mesh service canary path.
	MeshServiceCanaryPath = "/mesh/services/{serviceName:string}/canary"

	// MeshServiceResiliencePath is the mesh service resilience path.
	MeshServiceResiliencePath = "/mesh/services/{serviceName:string}/resilience"

	// MeshServiceLoadBalancePath is the mesh service load balance path.
	MeshServiceLoadBalancePath = "/mesh/services/{serviceName:string}/loadbalance"

	// MeshServiceOutputServerPath is the mesh service output server path.
	MeshServiceOutputServerPath = "/mesh/services/{serviceName:string}/outputserver"

	// MeshServiceTracingsPath is the mesh service tracings path.
	MeshServiceTracingsPath = "/mesh/services/{serviceName:string}/tracings"

	// MeshServiceMetricsPath is the mesh service metrics path.
	MeshServiceMetricsPath = "/mesh/services/{serviceName:string}/metrics"

	// MeshServiceInstancePrefix is the mesh service prefix.
	MeshServiceInstancePrefix = "/mesh/serviceinstances"

	// MeshServiceInstancePath is the mesh service path.
	MeshServiceInstancePath = "/mesh/serviceinstances/{serviceName:string}/{instanceID:string}"
)

func (m *Master) registerAPIs() {
	meshAPIs := []*api.APIEntry{
		{Path: MeshTenantPrefix, Method: "GET", Handler: m.listTenants},
		{Path: MeshTenantPath, Method: "POST", Handler: m.createTenant},
		{Path: MeshTenantPath, Method: "GET", Handler: m.getTenant},
		{Path: MeshTenantPath, Method: "PUT", Handler: m.updateTenant},
		{Path: MeshTenantPath, Method: "DELETE", Handler: m.deleteTenant},

		{Path: MeshServicePrefix, Method: "GET", Handler: m.listServices},
		{Path: MeshServicePath, Method: "POST", Handler: m.createService},
		{Path: MeshServicePath, Method: "GET", Handler: m.getService},
		{Path: MeshServicePath, Method: "PUT", Handler: m.updateService},
		{Path: MeshServicePath, Method: "DELETE", Handler: m.deleteService},

		{Path: MeshServiceInstancePrefix, Method: "GET", Handler: m.listServiceInstanceSpecs},
		{Path: MeshServiceInstancePath, Method: "GET", Handler: m.getServiceInstanceSpec},
		{Path: MeshServiceInstancePath, Method: "DELETE", Handler: m.offlineSerivceInstance},

		{Path: MeshServiceCanaryPath, Method: "POST", Handler: m.createPartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "GET", Handler: m.getPartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "PUT", Handler: m.updatePartOfService(canaryMeta)},
		{Path: MeshServiceCanaryPath, Method: "DELETE", Handler: m.deletePartOfService(canaryMeta)},

		{Path: MeshServiceResiliencePath, Method: "POST", Handler: m.createPartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "GET", Handler: m.getPartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "PUT", Handler: m.updatePartOfService(resilienceMeta)},
		{Path: MeshServiceResiliencePath, Method: "DELETE", Handler: m.deletePartOfService(resilienceMeta)},

		{Path: MeshServiceLoadBalancePath, Method: "POST", Handler: m.createPartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "GET", Handler: m.getPartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "PUT", Handler: m.updatePartOfService(loadBalanceMeta)},
		{Path: MeshServiceLoadBalancePath, Method: "DELETE", Handler: m.deletePartOfService(loadBalanceMeta)},

		{Path: MeshServiceOutputServerPath, Method: "POST", Handler: m.createPartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "GET", Handler: m.getPartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "PUT", Handler: m.updatePartOfService(outputServerMeta)},
		{Path: MeshServiceOutputServerPath, Method: "DELETE", Handler: m.deletePartOfService(outputServerMeta)},

		{Path: MeshServiceTracingsPath, Method: "POST", Handler: m.createPartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "GET", Handler: m.getPartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "PUT", Handler: m.updatePartOfService(tracingsMeta)},
		{Path: MeshServiceTracingsPath, Method: "DELETE", Handler: m.deletePartOfService(tracingsMeta)},

		{Path: MeshServiceMetricsPath, Method: "POST", Handler: m.createPartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "GET", Handler: m.getPartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "PUT", Handler: m.updatePartOfService(metricsMeta)},
		{Path: MeshServiceMetricsPath, Method: "DELETE", Handler: m.deletePartOfService(metricsMeta)},
	}

	api.GlobalServer.RegisterAPIs(meshAPIs)
}

func (m *Master) readSpec(ctx iris.Context, spec interface{}) error {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	err = yaml.Unmarshal(body, spec)
	if err != nil {
		return fmt.Errorf("unmarshal %#v to yaml: %v", spec, err)
	}

	vr := v.Validate(spec, body)
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr)
	}

	return nil
}
