/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package registrycenter

import (
	"fmt"
	"strings"

	"github.com/ArthurHlt/go-eureka-client/eureka"
)

// ToEurekaInstanceInfo transforms service registry info to eureka's instance
func (rcs *Server) ToEurekaInstanceInfo(serviceInfo *ServiceRegistryInfo) *eureka.InstanceInfo {
	var ins eureka.InstanceInfo

	ins.HostName = serviceInfo.Ins.IP
	ins.IpAddr = serviceInfo.Ins.IP
	ins.App = strings.ToUpper(serviceInfo.Service.Name)
	ins.Status = eureka.UP
	ins.InstanceID = serviceInfo.Ins.InstanceID
	ins.DataCenterInfo = &eureka.DataCenterInfo{
		Name:  "MyOwn",
		Class: "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
	}
	ins.VipAddress = serviceInfo.Service.Name
	ins.SecureVipAddress = serviceInfo.Service.Name
	ins.ActionType = "ADDED"

	ins.Port = &eureka.Port{
		Enabled: true,
		Port:    int(serviceInfo.Ins.Port),
	}

	return &ins
}

// ToEurekaApp transforms registry center's service info to eureka's app
func (rcs *Server) ToEurekaApp(serviceInfo *ServiceRegistryInfo) *eureka.Application {
	var app eureka.Application

	app.Name = strings.ToUpper(serviceInfo.Service.Name)
	app.Instances = append(app.Instances, *rcs.ToEurekaInstanceInfo(serviceInfo))

	return &app
}

// ToEurekaApps transforms registry center's service info to eureka's apps
func (rcs *Server) ToEurekaApps(serviceInfos []*ServiceRegistryInfo) *eureka.Applications {
	var apps eureka.Applications
	for _, v := range serviceInfos {
		app := rcs.ToEurekaApp(v)
		apps.Applications = append(apps.Applications, *app)
		if apps.VersionsDelta == 0 {
			apps.VersionsDelta = int(v.Version)
		}
	}
	// according to eureka's client populateInstanceCountMap function
	apps.AppsHashcode = fmt.Sprintf("%s_%d_", eureka.UP, len(serviceInfos))

	return &apps
}
