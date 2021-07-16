/*
 * Copyright (c) 2017, MegaEase
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

package jmxtool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	yamljsontool "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	canaryConfigURL  = "/config-canary"
	serviceConfigURL = "/config-service"
)

type AgentInterface interface {
	UpdateService(newService *spec.Service, version int64) error
	UpdateCanary(globalHeaders *spec.GlobalCanaryHeaders, version int64) error
}

type AgentClient struct {
	URL        string
	HTTPClient *http.Client
}

func NewAgentClient(host, port string) *AgentClient {
	return &AgentClient{
		"http://" + host + ":" + port,
		&http.Client{},
	}
}

func (agent *AgentClient) UpdateService(newService *spec.Service, version int64) error {
	buff, err := yaml.Marshal(newService)
	if err != nil {
		return fmt.Errorf("marshal %#v to yaml failed: %v", newService, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}
	kvMap, err := JSONToKVMap(string(jsonBytes))
	kvMap["version"] = strconv.FormatInt(version, 10)

	bytes, err := json.Marshal(kvMap)
	if err != nil {
		return fmt.Errorf("marshal %s to json failed: %v", kvMap, err)
	}

	url := agent.URL + serviceConfigURL
	resp, err := handleRequest(http.MethodPut, url, bytes)
	logger.Infof("Update Service, URL: %s,request: %s, result: %v", url, string(bytes), resp)
	return err
}

func (agent *AgentClient) UpdateCanary(globalHeaders *spec.GlobalCanaryHeaders, version int64) error {
	buff, err := yaml.Marshal(globalHeaders)
	if err != nil {
		return fmt.Errorf("marshal %#v to yaml failed: %v", globalHeaders, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}
	kvMap, err := JSONToKVMap(string(jsonBytes))
	kvMap["version"] = strconv.FormatInt(version, 10)

	bytes, err := json.Marshal(kvMap)
	if err != nil {
		return fmt.Errorf("marshal %s to json failed: %v", kvMap, err)
	}

	url := agent.URL + canaryConfigURL
	resp, err := handleRequest(http.MethodPut, url, bytes)
	logger.Infof("Update Canary, URL: %s, request %s, result: %v", url, string(bytes), resp)
	return err
}
