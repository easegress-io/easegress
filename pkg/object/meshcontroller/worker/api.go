package worker

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshConsulPrefix is the mesh  consul registry center prefix.
	MeshConsulPrefix = "/mesh/registry/consul"

	// MeshEurekaPrefix is the mesh eureka registry center prefix.
	MeshEurekaPrefix = "/mesh/registry/eureka"
)

func (w *Worker) registerAPIs() {
	meshWorkerAPIs := []*api.APIEntry{
		// for consul registry/discovery RESTful APIs
		{
			Path:    MeshConsulPrefix + "/v1/catalog/register",
			Method:  "PUT",
			Handler: w.registry,
		},
		{
			Path:    MeshConsulPrefix + "/v1/catalog/deregister",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshConsulPrefix + "/v1/catalog/services",
			Method:  "GET",
			Handler: w.catalogServices,
		},
		{
			Path:    MeshConsulPrefix + "/v1/catalog/service/{serviceName:string}",
			Method:  "GET",
			Handler: w.catalogService,
		},

		// Eureka registry/discovery RESTful APIs
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}",
			Method:  "POST",
			Handler: w.registry,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/apps",
			Method:  "GET",
			Handler: w.apps,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}",
			Method:  "GET",
			Handler: w.app,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "GET",
			Handler: w.getAppInstance,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/instances/{instanceID:string}",
			Method:  "GET",
			Handler: w.getInstance,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/status",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/status",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/metadata",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/vips/{vipAddress:string}",
			Method:  "GET",
			Handler: w.emptyHandler,
		},
		{
			Path:    MeshEurekaPrefix + "/svips/{svipAddress:string}",
			Method:  "GET",
			Handler: w.emptyHandler,
		},
	}

	api.GlobalServer.RegisterAPIs(meshWorkerAPIs)
}

func (w *Worker) registry(ctx iris.Context) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	ins, err := w.registryServer.DecodeRegistryBody(body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	service := w.service.GetServiceSpec(ins.ServiceName)
	if service == nil {
		err := fmt.Errorf("registry to unknown service %s", ins.ServiceName)
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	if id, err := w.registryServer.Registry(ins, service, w.ingressServer.Ready, w.egressServer.Ready); err == nil {
		w.mutex.Lock()
		defer w.mutex.Unlock()
		w.instanceID = id
	} else if err != spec.ErrAlreadyRegistered {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	// NOTE: According to eureka APIs list:
	// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
	if w.registryServer.RegistryType == spec.RegistryTypeEureka {
		ctx.StatusCode(http.StatusNoContent)
	}
}

func (w *Worker) catalogServices(ctx iris.Context) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = w.registryServer.Discovery(); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	catalogServices := w.registryServer.ToConsulServices(serviceInfos)

	buff := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buff)
	if err := enc.Encode(catalogServices); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff.Bytes())
}

func (w *Worker) catalogService(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("empty service name"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = w.registryServer.DiscoveryService(serviceName); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	catalogService := w.registryServer.ToConsulCatalogService(serviceInfo)

	buff := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buff)
	if err := enc.Encode(catalogService); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff.Bytes())
}

func (w *Worker) apps(ctx iris.Context) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = w.registryServer.Discovery(); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	apps := w.registryServer.ToEurekaApps(serviceInfos)
	buff, err := xml.Marshal(apps)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)
}

func (w *Worker) app(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("empty service name(app)"))
		return
	}
	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = w.registryServer.DiscoveryService(serviceName); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	app := w.registryServer.ToEurekaApp(serviceInfo)
	buff, err := xml.Marshal(app)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)
}

func (w *Worker) emptyHandler(ctx iris.Context) {
	// EaseMesh does not need to implement some APIS like
	// delete, heartbeat of Eureka.
}

func (w *Worker) getAppInstance(ctx iris.Context) {
	serviceName := ctx.Params().Get("serviceName")
	if serviceName == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("empty service name(app)"))
		return
	}
	instanceID := ctx.Params().Get("instanceID")
	if instanceID == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("empty instanceID"))
		return
	}

	serviceInfo, err := w.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	if serviceInfo.Service.Name == serviceName && instanceID == serviceInfo.Ins.InstanceID {
		ins := w.registryServer.ToEurekaInstanceInfo(serviceInfo)
		buff, err := xml.Marshal(ins)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
		ctx.Header("Content-Type", "text/xml")
		ctx.Write(buff)
	}

	ctx.StatusCode(http.StatusNotFound)
}

func (w *Worker) getInstance(ctx iris.Context) {
	instanceID := ctx.Params().Get("instanceID")
	if instanceID == "" {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("empty instanceID"))
		return
	}
	serviceName := registrycenter.GetServiceName(instanceID)
	if len(serviceName) == 0 {
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("unknow instanceID:%s", instanceID))
		return
	}

	serviceInfo, err := w.registryServer.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ins := w.registryServer.ToEurekaInstanceInfo(serviceInfo)
	buff, err := xml.Marshal(ins)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)
}
