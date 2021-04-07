package worker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
)

func (w *Worker) consulAPIs() []*apiEntry {
	APIs := []*apiEntry{
		{
			Path:    "/v1/catalog/register",
			Method:  "PUT",
			Handler: w.consulRegister,
		},
		{
			Path:    "/v1/agent/service/register",
			Method:  "PUT",
			Handler: w.consulRegister,
		},
		{
			Path:    "/v1/agent/service/deregister",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    "/v1/health/service/{serviceName:string}",
			Method:  "GET",
			Handler: w.healthService,
		},
		{
			Path:    "/v1/catalog/deregister",
			Method:  "DELETE",
			Handler: w.emptyHandler,
		},
		{
			Path:    "/v1/catalog/services",
			Method:  "GET",
			Handler: w.catalogServices,
		},
		{
			Path:    "/v1/catalog/service/{serviceName:string}",
			Method:  "GET",
			Handler: w.catalogService,
		},
	}

	return APIs
}

func (w *Worker) consulRegister(ctx iris.Context) {
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
}

func (w *Worker) healthService(ctx iris.Context) {
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

	serviceEntry := w.registryServer.ToConsulHealthService(serviceInfo)

	buff, err := json.Marshal(serviceEntry)
	if err != nil {
		logger.Errorf("json marshal sericeEntry: %#v err: %v", serviceEntry, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", registrycenter.ContentTypeJSON)
	ctx.Write(buff)
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
		logger.Errorf("discovery service: %s, err: %v ", serviceName, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	catalogService := w.registryServer.ToConsulCatalogService(serviceInfo)

	buff, err := json.Marshal(catalogService)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		logger.Errorf("json marshal catalogService: %#v err: %v", catalogService, err)
		return
	}

	ctx.Header("Content-Type", registrycenter.ContentTypeJSON)
	ctx.Write(buff)
}

func (w *Worker) catalogServices(ctx iris.Context) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = w.registryServer.Discovery(); err != nil {
		logger.Errorf("discovery services err: %v ", err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	catalogServices := w.registryServer.ToConsulServices(serviceInfos)

	buff, err := json.Marshal(catalogServices)
	if err != nil {
		logger.Errorf("json marshal catalogServices: %#v err: %v", catalogServices, err)
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", registrycenter.ContentTypeJSON)
	ctx.Write(buff)
}
