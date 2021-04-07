package worker

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshEurekaPrefix is the mesh eureka registry center prefix.
	MeshEurekaPrefix = "/mesh/eureka"

	contentTypeXML  = "text/xml"
	contentTypeJSON = "application/json"
)

func (w *Worker) registerAPIs() {
	var apis []*apiEntry
	switch w.registryServer.RegistryType {
	case spec.RegistryTypeConsul:
		apis = w.consulAPIs()
	case spec.RegistryTypeEureka:
		apis = w.eurekaAPIs()
	default:
		apis = w.eurekaAPIs()
	}
	w.apiServer.registerAPIs(apis)
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
		err := fmt.Errorf("registry to unknown service: %s", w.serviceName)
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

func (w *Worker) emptyHandler(ctx iris.Context) {
	// EaseMesh does not need to implement some APIS like
	// delete, heartbeat of Eureka.
}
