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
)

type (
	eurekaJSONApps struct {
		APPs eurekaAPPs `json:"applications"`
	}

	eurekaAPPs struct {
		VersionDelta string      `json:"versions__delta"`
		AppHashCode  string      `json:"apps__hashcode"`
		Application  []eurekaAPP `json:"application"`
	}

	eurekaJSONAPP struct {
		APP eurekaAPP `json:"application"`
	}

	eurekaAPP struct {
		Name      string                `json:"name"`
		Instances []eureka.InstanceInfo `json:"instance"`
	}
)

func (w *Worker) eurekaAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}",
			Method:  "POST",
			Handler: w.eurekaRegister,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/",
			Method:  "GET",
			Handler: w.apps,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}",
			Method:  "GET",
			Handler: w.app,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "GET",
			Handler: w.getAppInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/instances/{instanceID:string}",
			Method:  "GET",
			Handler: w.getInstance,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/status",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/status",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		}, {
			Path:    meshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/metadata",
			Method:  "PUT",
			Handler: w.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/vips/{vipAddress:string}",
			Method:  "GET",
			Handler: w.emptyHandler,
		},
		{
			Path:    meshEurekaPrefix + "/svips/{svipAddress:string}",
			Method:  "GET",
			Handler: w.emptyHandler,
		},
	}

	return APIs
}

func (w *Worker) eurekaRegister(ctx iris.Context) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("read body failed: %v", err))
		return
	}
	contentType := ctx.Request().Header.Get("Content-Type")
	if err := w.registryServer.CheckRegistryBody(contentType, body); err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceSpec := w.service.GetServiceSpec(w.serviceName)
	if serviceSpec == nil {
		err := fmt.Errorf("registry to unknown service: %s", w.serviceName)
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	w.registryServer.Register(serviceSpec, w.ingressServer.Ready, w.egressServer.Ready)

	// NOTE: According to eureka APIs list:
	// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
	ctx.StatusCode(http.StatusNoContent)
}

func (w *Worker) apps(ctx iris.Context) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = w.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	xmlAPPs := w.registryServer.ToEurekaApps(serviceInfos)
	jsonAPPs := eurekaJSONApps{
		APPs: eurekaAPPs{
			VersionDelta: strconv.Itoa(xmlAPPs.VersionsDelta),
			AppHashCode:  xmlAPPs.AppsHashcode,
		},
	}

	for _, v := range xmlAPPs.Applications {
		jsonAPPs.APPs.Application = append(jsonAPPs.APPs.Application, eurekaAPP{Name: v.Name, Instances: v.Instances})
	}

	accept := ctx.Request().Header.Get("Accept")

	rsp, err := w.encodByAcceptType(accept, jsonAPPs, xmlAPPs)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
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

	// eureka use 'delta' after /apps/, need to handle this
	// special case here.
	if serviceName == "delta" {
		w.apps(ctx)
		return
	}

	var (
		err         error
		serviceInfo *registrycenter.ServiceRegistryInfo
	)

	if serviceInfo, err = w.registryServer.DiscoveryService(serviceName); err != nil {
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	accept := ctx.Request().Header.Get("Accept")
	xmlAPP := w.registryServer.ToEurekaApp(serviceInfo)

	jsonApp := eurekaJSONAPP{
		APP: eurekaAPP{
			Name:      xmlAPP.Name,
			Instances: xmlAPP.Instances,
		},
	}
	rsp, err := w.encodByAcceptType(accept, jsonApp, xmlAPP)
	if err != nil {
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", accept)
	ctx.Write([]byte(rsp))
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
			logger.Errorf("encode accept: %s failed: %v", accept, err)
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
		api.HandleAPIError(ctx, http.StatusBadRequest, fmt.Errorf("unknown instanceID: %s", instanceID))
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
		logger.Errorf("encode accept: %s failed: %v", accept, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Header("Content-Type", accept)
	ctx.Write([]byte(rsp))
}

func (w *Worker) encodByAcceptType(accept string, jsonSt interface{}, xmlSt interface{}) ([]byte, error) {
	switch accept {
	case registrycenter.ContentTypeJSON:
		buff := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buff)
		err := enc.Encode(jsonSt)
		return buff.Bytes(), err
	default:
		buff, err := xml.Marshal(xmlSt)
		return buff, err
	}
}
