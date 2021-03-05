package worker

import (
	"fmt"
	"io/ioutil"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registry"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
)

// Registry is a HTTP handler for worker, handling
//  java business process's Eureka/Consul registry RESTful request
func (w *Worker) Registry(ctx iris.Context) error {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}
	ins, err := w.rcs.DecodeBody(body)
	if err != nil {
		return err
	}
	serviceYAML, err := w.store.Get(layout.GenServerKey(w.serviceName))
	if err != nil {
		return err
	}
	var service spec.Service
	if err = yaml.Unmarshal([]byte(*serviceYAML), &service); err != nil {
		return err
	}

	if ID, port, err := w.rcs.RegistryServiceInstance(ins, &service, w.ings.CheckIngressReady); err == nil {
		w.mux.Lock()
		defer w.mux.Unlock()
		// let worker know its instance identity
		w.instanceID = ID
		// asynchronous create ingress
		go w.createIngress(&service, port)
	} else {
		if err != registry.ErrAlreadyRegistried {
			ctx.StatusCode(iris.StatusInternalServerError)
			return err
		}
	}

	return nil
}
