package registrycenter

import "github.com/ArthurHlt/go-eureka-client/eureka"

// ToEurekaInstanceInfo transfors serivce registry info to eureka's instance
func (rcs *Server) ToEurekaInstanceInfo(serviceInfo *ServiceRegistryInfo) *eureka.InstanceInfo {
	var ins eureka.InstanceInfo

	ins.HostName = serviceInfo.Service.EgressAddr()
	ins.IpAddr = serviceInfo.Ins.IP
	ins.App = serviceInfo.Service.Name
	ins.Status = eureka.UP
	ins.InstanceID = serviceInfo.Ins.InstanceID

	ins.Port = &eureka.Port{
		Enabled: true,
		Port:    int(serviceInfo.Ins.Port),
	}

	return &ins

}

// ToEurekaApp transfors registry center's serivce info to eureka's app
func (rcs *Server) ToEurekaApp(serviceInfo *ServiceRegistryInfo) *eureka.Application {
	var app eureka.Application

	app.Name = serviceInfo.Service.Name
	app.Instances = append(app.Instances, *rcs.ToEurekaInstanceInfo(serviceInfo))

	return &app
}

// ToEurekaApps transfors registry center's serivce info to eureka's apps
func (rcs *Server) ToEurekaApps(serviceInfos []*ServiceRegistryInfo) *eureka.Applications {
	var apps eureka.Applications
	delta := ""
	for _, v := range serviceInfos {
		app := rcs.ToEurekaApp(v)
		delta += eureka.UP + "_1_" // according to eureka's client populateInstanceCountMap function
		apps.Applications = append(apps.Applications, *app)
	}
	apps.AppsHashcode = delta

	return &apps
}
