package registrycenter

import (
	"fmt"
	"strings"

	"github.com/ArthurHlt/go-eureka-client/eureka"
)

// ToEurekaInstanceInfo transforms serivce registry info to eureka's instance
func (rcs *Server) ToEurekaInstanceInfo(serviceInfo *ServiceRegistryInfo) *eureka.InstanceInfo {
	var ins eureka.InstanceInfo

	ins.HostName = serviceInfo.Service.EgressEndpoint()
	ins.IpAddr = serviceInfo.Ins.IP
	ins.App = strings.ToUpper(serviceInfo.Service.Name)
	ins.Status = eureka.UP
	ins.InstanceID = serviceInfo.Ins.InstanceID
	ins.ActionType = "ADDED"

	ins.Port = &eureka.Port{
		Enabled: true,
		Port:    int(serviceInfo.Ins.Port),
	}

	return &ins

}

// ToEurekaApp transforms registry center's serivce info to eureka's app
func (rcs *Server) ToEurekaApp(serviceInfo *ServiceRegistryInfo) *eureka.Application {
	var app eureka.Application

	app.Name = strings.ToUpper(serviceInfo.Service.Name)
	app.Instances = append(app.Instances, *rcs.ToEurekaInstanceInfo(serviceInfo))

	return &app
}

// ToEurekaApps transforms registry center's serivce info to eureka's apps
func (rcs *Server) ToEurekaApps(serviceInfos []*ServiceRegistryInfo) *eureka.Applications {
	var apps eureka.Applications
	for _, v := range serviceInfos {
		app := rcs.ToEurekaApp(v)
		apps.Applications = append(apps.Applications, *app)
	}
	// according to eureka's client populateInstanceCountMap function
	apps.AppsHashcode = fmt.Sprintf("%s_%d_", eureka.UP, len(serviceInfos))

	return &apps
}

// ToEurekaAppsDelta transforms registry center's service info to eureka's apps delta
func (rcs *Server) ToEurekaAppsDelta(serviceInfos []*ServiceRegistryInfo) *eureka.Applications {
	var apps eureka.Applications
	apps.AppsHashcode = fmt.Sprintf("%s_%d_", eureka.UP, len(serviceInfos))
	return &apps
}
