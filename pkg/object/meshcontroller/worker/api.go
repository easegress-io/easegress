package worker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/kataras/iris"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"
)

func (w *Worker) registerAPIs() {
	meshWorkerAPIs := []*api.APIEntry{
		{
			// for consule put RESTful API
			Path:    MeshPrefix,
			Method:  "PUT",
			Handler: w.registry,
		},
		{
			// for eureka POST RESTful API
			Path:    MeshPrefix,
			Method:  "POST",
			Handler: w.registry,
		},
	}

	api.GlobalServer.RegisterAPIs(meshWorkerAPIs)
}

// createIngress calls ingress server create default HTTPServer and pipeline
// loop until succ
func (w *Worker) createIngress(service *spec.Service, port uint32) {
	var err error
	for {
		if err = w.ings.createIngress(service, port); err != nil {
			logger.Errorf("worker create ingress failed: %v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	return
}

// Registry is a HTTP handler for worker, handling
// java business process's Eureka/Consul registry RESTful request
func (w *Worker) registry(ctx iris.Context) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("registry read body failed: %v", err))
		return
	}
	ins, err := w.rcs.DecodeRegistryBody(body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	serviceYAML, err := w.store.Get(layout.ServiceKey(w.serviceName))
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	if serviceYAML == nil {
		err := fmt.Errorf("worker registry into not exist service :%s", w.serviceName)
		api.HandleAPIError(ctx, http.StatusBadRequest, err)
		return
	}

	var service spec.Service
	if err = yaml.Unmarshal([]byte(*serviceYAML), &service); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	// asynchronous create ingress
	go w.createIngress(&service, ins.Port)
	if ID, err := w.rcs.RegistryServiceInstance(ins, &service, w.ings.CheckIngressReady); err == nil {
		w.mux.Lock()
		defer w.mux.Unlock()
		// let worker know its instance identity
		w.instanceID = ID
	} else {
		if err != registrycenter.ErrAlreadyRegistried {
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	return
}

// Discovery is a HTTP handler for worker, handling
// java business process's Eureka/Consul discovery RESTful request
func (w *Worker) Discovery(ctx iris.Context) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusBadRequest,
			fmt.Errorf("discovery read body failed: %v", err))
		return
	}

	// [TODO] call registrycenter's discovery implement
	logger.Debugf("discovery request body [%s]", body)

	return
}
