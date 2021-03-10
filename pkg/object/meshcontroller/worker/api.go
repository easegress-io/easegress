package worker

import (
	"encoding/xml"
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

	// MeshEurekaPrefix is the mesh eureka registry center prefix.
	MeshEurekaPrefix = "/mesh/registry/eureka"
)

func (w *Worker) registerAPIs() {
	meshWorkerAPIs := []*api.APIEntry{
		{
			// for consule put RESTful API
			Path:    MeshPrefix,
			Method:  "PUT",
			Handler: w.registry,
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
			Handler: w.emptyImplement,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}",
			Method:  "PUT",
			Handler: w.emptyImplement,
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
			Handler: w.emptyImplement,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/status",
			Method:  "DELETE",
			Handler: w.emptyImplement,
		},
		{
			Path:    MeshEurekaPrefix + "/apps/{serviceName:string}/{instanceID:string}/metadata",
			Method:  "PUT",
			Handler: w.emptyImplement,
		},
		{
			Path:    MeshEurekaPrefix + "/vips/{vipAddress:string}",
			Method:  "GET",
			Handler: w.emptyImplement,
		},
		{
			Path:    MeshEurekaPrefix + "/svips/{svipAddress:string}",
			Method:  "GET",
			Handler: w.emptyImplement,
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

func (w *Worker) createEgress(service *spec.Service) {
	var err error
	for {
		if err = w.egs.createEgress(service); err != nil {
			logger.Errorf("worker create egress failed: %v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
}

// getSerivceInstances get whole service Instances from store.
func (w *Worker) getSerivceInstances(serviceName string) ([]*spec.ServiceInstance, error) {
	var insList []*spec.ServiceInstance

	insYAMLs, err := w.store.GetPrefix(layout.ServiceInstancePrefix(serviceName))
	if err != nil {
		return insList, err
	}

	for k, v := range insYAMLs {
		var ins *spec.ServiceInstance
		if err = yaml.Unmarshal([]byte(v), ins); err != nil {
			logger.Errorf("BUG unmarsh service :%s,  instanceID:%s , val:%s failed, err:%v", serviceName, k, v, err)
			continue
		}
		insList = append(insList, ins)
	}

	return insList, nil

}

// registry is a HTTP handler for worker, handling
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

	// asynchronous create ingress/egress
	go w.createIngress(&service, ins.Port)
	go w.createEgress(&service)

	if ID, err := w.rcs.RegistryServiceInstance(ins, &service, w.ings.Ready, w.egs.Ready); err == nil {
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

	// according to eureka APIs list
	// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
	if w.rcs.RegistryType == spec.RegistryTypeEureka {
		ctx.StatusCode(http.StatusNoContent)
	}
	return
}

func (w *Worker) apps(ctx iris.Context) {
	var (
		err          error
		serviceInfos []*registrycenter.ServiceRegistryInfo
	)
	if serviceInfos, err = w.rcs.Discovery(); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	apps := w.rcs.ToEurekaApps(serviceInfos)
	buff, err := xml.Marshal(apps)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)

	return
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

	if serviceInfo, err = w.rcs.DiscoveryService(serviceName); err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	// create egress
	if serviceName != w.serviceName {
		ins, err := w.getSerivceInstances(serviceName)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
		if err = w.egs.addEgress(serviceInfo.Service, ins); err != nil {
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	app := w.rcs.ToEurekaApp(serviceInfo)
	buff, err := xml.Marshal(app)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)

	return
}

func (w *Worker) emptyImplement(ctx iris.Context) {
	// empty implement, easemesh don't need to implement
	// this eurka API, including, delete, heartbeat
	return
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

	serviceInfo, err := w.rcs.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}

	if serviceInfo.Service.Name == serviceName && instanceID == serviceInfo.Ins.InstanceID {
		ins := w.rcs.ToEurekaInstanceInfo(serviceInfo)
		buff, err := xml.Marshal(ins)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusInternalServerError, err)
			return
		}
		ctx.Header("Content-Type", "text/xml")
		ctx.Write(buff)
	}

	ctx.StatusCode(http.StatusNotFound)
	return
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

	serviceInfo, err := w.rcs.DiscoveryService(serviceName)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ins := w.rcs.ToEurekaInstanceInfo(serviceInfo)
	buff, err := xml.Marshal(ins)
	if err != nil {
		api.HandleAPIError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Header("Content-Type", "text/xml")
	ctx.Write(buff)
	return
}
