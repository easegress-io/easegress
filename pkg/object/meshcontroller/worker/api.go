package worker

import (
	"github.com/kataras/iris"

	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
)

const (
	// meshEurekaPrefix is the mesh eureka registry API url prefix.
	meshEurekaPrefix = "/mesh/eureka"

	// meshNacosPrefix is the mesh nacos registyr API url prefix.
	meshNacosPrefix = "/nacos/v1"
)

func (w *Worker) runAPIServer() {
	var apis []*apiEntry
	switch w.registryServer.RegistryType {
	case spec.RegistryTypeConsul:
		apis = w.consulAPIs()
	case spec.RegistryTypeEureka:
		apis = w.eurekaAPIs()
	case spec.RegistryTypeNacos:
		apis = w.nacosAPIs()
	default:
		apis = w.eurekaAPIs()
	}
	w.apiServer.registerAPIs(apis)
	go w.apiServer.run()
}

func (w *Worker) emptyHandler(ctx iris.Context) {
	// EaseMesh does not need to implement some APIS like
	// delete, heartbeat of Eureka/Consul/Nacos.
}
