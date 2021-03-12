package master

import (
	"fmt"
	"net/http"

	"github.com/kataras/iris"
	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
)

type (
	newPartFunc func() interface{}
	partOfFunc  func(serviceSpec *spec.Service) (interface{}, bool)
	setPartFunc func(serviceSpec *spec.Service, part interface{})

	partMeta struct {
		partName string
		newPart  newPartFunc
		partOf   partOfFunc
		setPart  setPartFunc
	}
)

var (
	canaryMeta = &partMeta{
		partName: "canary",
		newPart: func() interface{} {
			return &spec.Canary{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			return serviceSpec.Canary, serviceSpec.Canary != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			serviceSpec.Canary = part.(*spec.Canary)
		},
	}

	loadBalanceMeta = &partMeta{
		partName: "loadBalance",
		newPart: func() interface{} {
			return &spec.LoadBalance{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			return serviceSpec.LoadBalance, serviceSpec.LoadBalance != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			serviceSpec.LoadBalance = part.(*spec.LoadBalance)
		},
	}

	outputServerMeta = &partMeta{
		partName: "outputServer",
		newPart: func() interface{} {
			return &spec.ObservabilityOutputServer{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.OutputServer, serviceSpec.Observability.OutputServer != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			serviceSpec.Observability.OutputServer = part.(*spec.ObservabilityOutputServer)
		},
	}

	tracingsMeta = &partMeta{
		partName: "tracings",
		newPart: func() interface{} {
			return &spec.ObservabilityTracings{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.Tracings, serviceSpec.Observability.Tracings != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			serviceSpec.Observability.Tracings = part.(*spec.ObservabilityTracings)
		},
	}

	metricsMeta = &partMeta{
		partName: "metrics",
		newPart: func() interface{} {
			return &spec.ObservabilityMetrics{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.Metrics, serviceSpec.Observability.Metrics != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			serviceSpec.Observability.Metrics = part.(*spec.ObservabilityMetrics)
		},
	}
)

func (m *Master) getPartOfService(meta *partMeta) iris.Handler {
	return func(ctx iris.Context) {
		serviceName, err := m.readServiceName(ctx)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		// NOTE: No need to lock.
		serviceSpec := m.service.getServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		part, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("%s of service %s not found", meta.partName, serviceName))
		}

		buff, err := yaml.Marshal(part)
		if err != nil {
			panic(err)
		}
		ctx.Header("Content-Type", "text/vnd.yaml")
		ctx.Write(buff)
	}
}

func (m *Master) createPartOfService(meta *partMeta) iris.Handler {
	return func(ctx iris.Context) {
		serviceName, err := m.readServiceName(ctx)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		part := meta.newPart()
		err = m.readSpec(ctx, part)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		m.storageLock()
		defer m.storageUnlock()

		serviceSpec := m.service.getServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if existed {
			api.HandleAPIError(ctx, http.StatusConflict,
				fmt.Errorf("%s of service existed", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, part)

		m.service.putServiceSpec(serviceSpec)

		ctx.Header("Location", ctx.Path())
		ctx.StatusCode(http.StatusCreated)
	}
}

func (m *Master) updatePartOfService(meta *partMeta) iris.Handler {
	return func(ctx iris.Context) {
		serviceName, err := m.readServiceName(ctx)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		part := meta.newPart()
		err = m.readSpec(ctx, part)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		m.storageLock()
		defer m.storageUnlock()

		serviceSpec := m.service.getServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("%s of service found", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, part)
		m.service.putServiceSpec(serviceSpec)
	}
}

func (m *Master) deletePartOfService(meta *partMeta) iris.Handler {
	return func(ctx iris.Context) {
		serviceName, err := m.readServiceName(ctx)
		if err != nil {
			api.HandleAPIError(ctx, http.StatusBadRequest, err)
			return
		}

		m.storageLock()
		defer m.storageUnlock()

		serviceSpec := m.service.getServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(ctx, http.StatusNotFound,
				fmt.Errorf("%s of service found", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, nil)
		m.service.putServiceSpec(serviceSpec)
	}
}
