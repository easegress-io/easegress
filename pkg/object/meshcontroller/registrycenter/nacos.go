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
	"time"

	"github.com/nacos-group/nacos-sdk-go/model"
)

const defaultGroup = "DEFAULT_GROUP"

// ToNacosInstanceInfo transforms service registry info to nacos' instance
func (rcs *Server) ToNacosInstanceInfo(serviceInfo *ServiceRegistryInfo) *model.Instance {
	var ins model.Instance

	ins.Ip = serviceInfo.Ins.IP
	ins.Valid = true
	ins.InstanceId = serviceInfo.Ins.InstanceID
	ins.ServiceName = defaultGroup + "@@" + serviceInfo.Ins.ServiceName
	ins.Port = uint64(serviceInfo.Ins.Port)
	ins.Healthy = true
	ins.Enable = true
	ins.Weight = 1.0
	ins.Ephemeral = true
	ins.ClusterName = "DEFAULT"

	return &ins
}

// ToNacosService transforms servie registry info to nacos' service
func (rcs *Server) ToNacosService(serviceInfo *ServiceRegistryInfo) *model.Service {
	var svc model.Service
	svc.Hosts = append(svc.Hosts, *rcs.ToNacosInstanceInfo(serviceInfo))
	svc.Dom = serviceInfo.Ins.ServiceName
	svc.CacheMillis = 500
	svc.Name = defaultGroup + "@@" + serviceInfo.Ins.ServiceName
	svc.LastRefTime = uint64(time.Now().Unix())
	return &svc
}

// ToNacosServiceList transforms registry center's service info to nacos' apps
func (rcs *Server) ToNacosServiceList(serviceInfos []*ServiceRegistryInfo) *model.ServiceList {
	var list model.ServiceList
	for _, v := range serviceInfos {
		list.Doms = append(list.Doms, v.Ins.ServiceName)
	}

	list.Count = int64(len(serviceInfos))
	return &list
}

// ToNacosServiceDetail transforms servie registry info to nacos' service
func (rcs *Server) ToNacosServiceDetail(serviceInfo *ServiceRegistryInfo) *model.ServiceDetail {
	var svc model.ServiceDetail
	svc.Service.Name = serviceInfo.Ins.ServiceName
	return &svc
}

// SplitNacosServiceName gets nacos servicename in GROUP_NAME@@SERVICE_NAME format
func (rcs *Server) SplitNacosServiceName(serviceName string) (string, error) {
	if strings.Contains(serviceName, "@@") {
		names := strings.Split(serviceName, "@@")
		if len(names) != 2 {
			return "", fmt.Errorf("invalid servicename: %s", serviceName)
		}

		return names[1], nil
	}

	return serviceName, nil
}
