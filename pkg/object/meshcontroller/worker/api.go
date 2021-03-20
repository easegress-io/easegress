package worker

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
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

	contentTypeXML  = "text/xml"
	contentTypeJSON = "application/json"
)

var (
	eurekaJSONTypeAPPs = "applications"
	eurekaJSONTypeAPP  = "application"
	eurekaJSONTypeIns  = "instance"
	eurekaXML          = ""
)

type (
	eurekaJSONApps struct {
		APPs eurekaAPPs `json:"applications"`
	}

	eurekaAPPs struct {
		VersionDelta string               `json:"versions__delat"`
		AppHashCode  string               `json:"apps__hashcode"`
		Application  []eureka.Application `json:"aplication"`
	}

	eurekaJSONAPP struct {
		APP eureka.Application `json:"application"`
	}
)

func (w *Worker) registerAPIs() {
	meshWorkerAPIs := []*api.APIEntry{
		// for consul registry/discovery RESTful APIs
		{
			Path:    MeshConsulPrefix + "/v1/catalog/register",
			Method:  "PUT",
			Handler: w.applicationRegister,
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
			Handler: w.applicationRegister,
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
			Path:    MeshEurekaPrefix + "/apps/",
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

	// Debug info in sidecar image
	fmt.Printf("api global servce is %#v", api.GlobalServer)
	api.GlobalServer.RegisterAPIs(meshWorkerAPIs)
}

func (w *Worker) applicationRegister(ctx iris.Context) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := ctx.Request().Header.Get("Content-Type")
	if err := w.registryServer.DecodeRegistryBody(contentType, body); err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := w.service.GetServiceSpec(w.serviceName)
	if serviceSpec == nil {
		err := fmt.Errorf("registry to unknown service %s", w.serviceName)
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	w.registryServer.Register(serviceSpec, w.ingressServer.Ready, w.egressServer.Ready)

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

	ctx.Header("Content-Type", contentTypeJSON)
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

	ctx.Header("Content-Type", contentTypeJSON)
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
	xmlAPPs := w.registryServer.ToEurekaApps(serviceInfos)

	jsonAPPs := eurekaJSONApps{
		APPs: eurekaAPPs{
			VersionDelta: strconv.Itoa(xmlAPPs.VersionsDelta),
			AppHashCode:  xmlAPPs.AppsHashcode,
			Application:  xmlAPPs.Applications,
		},
	}
	accept := ctx.Request().Header.Get("Accept")

	rsp, err := w.encodByAcceptType(accept, jsonAPPs, xmlAPPs)
	if err != nil {
		logger.Errorf("encode accept:%s, failed, err:%v", accept, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", accept)
	ctx.Write([]byte(rsp))
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

	accept := ctx.Request().Header.Get("Accept")
	xmlAPP := w.registryServer.ToEurekaApp(serviceInfo)

	jsonApp := eurekaJSONAPP{
		APP: *xmlAPP,
	}
	rsp, err := w.encodByAcceptType(accept, jsonApp, xmlAPP)
	if err != nil {
		logger.Errorf("encode accept:%s, failed, err:%v", accept, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", accept)
	ctx.Write([]byte(rsp))
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
		accept := ctx.Request().Header.Get("Accept")

		rsp, err := w.encodByAcceptType(accept, ins, ins)
		if err != nil {
			logger.Errorf("encode accept:%s, failed, err:%v", accept, err)
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
		ctx.Header("Content-Type", accept)
		ctx.Write([]byte(rsp))
		return
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
	accept := ctx.Request().Header.Get("Accept")

	rsp, err := w.encodByAcceptType(accept, ins, ins)
	if err != nil {
		logger.Errorf("encode accept:%s, failed, err:%v", accept, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Header("Content-Type", accept)
	ctx.Write([]byte(rsp))
}

func (w *Worker) encodByAcceptType(accept string, jsonSt interface{}, xmlSt interface{}) ([]byte, error) {
	switch accept {
	case contentTypeJSON:
		buff := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buff)
		err := enc.Encode(jsonSt)
		return buff.Bytes(), err
	default:
		buff, err := xml.Marshal(xmlSt)
		return buff, err
	}
}
