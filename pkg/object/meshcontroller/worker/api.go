package worker

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
)

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
//  java business process's Eureka/Consul registry RESTful request
func (w *Worker) Registry(ctx iris.Context) error {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		return fmt.Errorf("read body failed: %v", err)
	}
	ins, err := w.rcs.DecodeBody(body)
	if err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		return err
	}

	serviceYAML, err := w.store.Get(layout.GenServerKey(w.serviceName))
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		return err
	}
	if serviceYAML == nil {
		logger.Errorf("worker registry into not exist service :%s", w.serviceName)
		ctx.StatusCode(iris.StatusBadRequest)
		return err
	}
	var service spec.Service
	if err = yaml.Unmarshal([]byte(*serviceYAML), &service); err != nil {
		return err
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
			ctx.StatusCode(iris.StatusInternalServerError)
			return err
		}
	}
	return nil
}
