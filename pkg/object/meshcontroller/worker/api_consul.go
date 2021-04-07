package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
)

func (w *Worker) consulAPIs() []*apiEntry {
	APIs := []*apiEntry{
		// for consul registry/discovery RESTful APIs
		{
			Path:    "/v1/catalog/register",
			Method:  "PUT",
			Handler: w.applicationRegister,
		},
		{
			Path:    "/v1/agent/service/register",
			Method:  "PUT",
			Handler: w.applicationRegister,
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

	buff := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buff)
	if err := enc.Encode(serviceEntry); err != nil {
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
