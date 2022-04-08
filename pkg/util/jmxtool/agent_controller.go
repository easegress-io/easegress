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

	yamljsontool "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	agentConfigURL = "/config"
)

type (
	// AgentInterface is the interface operate the agent client.
	AgentInterface interface {
		UpdateAgentConfig(config *AgentConfig) error
	}

	// AgentClient stores the information of agent client
	AgentClient struct {
		URL        string
		HTTPClient *http.Client
	}

	// AgentConfig is the config pushed to agent.
	AgentConfig struct {
		spec.Service `yaml:",inline"`

		Headers  string         `yaml:"easeagent.progress.forwarded.headers"`
		Reporter *AgentReporter `yaml:"reporter.outputServer"`
	}

	// AgentReporter is the basic config for agent reporter.
	AgentReporter struct {
		ReporterTLS *AgentReporterTLS `yaml:"tls"`

		AppendType      string `yaml:"appendType"`
		BootstrapServer string `yaml:"bootstrapServer"`
		Username        string `yaml:"username"`
		Password        string `yaml:"password"`
	}

	// AgentReporterTLS is the TLS config for agent resporter.
	AgentReporterTLS struct {
		Enable bool   `yaml:"enable"`
		Key    string `yaml:"key"`
		Cert   string `yaml:"cert"`
		CACert string `yaml:"ca_cert"`
	}
)

func newAgentConfig() {
}

func (ac *AgentConfig) marshal() ([]byte, error) {
	yamlBuff, err := yaml.Marshal(ac)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to yaml failed: %v", ac, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(yamlBuff)
	if err != nil {
		return nil, fmt.Errorf("convert yaml %s to json failed: %v", yamlBuff, err)
	}
	kvMap, err := JSONToKVMap(string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("json to kv failed: %v", err)
	}

	result, err := json.Marshal(kvMap)
	if err != nil {
		return nil, fmt.Errorf("marshal %s to json failed: %v", kvMap, err)
	}

	return result, nil
}

// NewAgentClient creates the agent client
func NewAgentClient(host, port string) *AgentClient {
	return &AgentClient{
		"http://" + host + ":" + port,
		&http.Client{},
	}
}

// UpdateAgentConfig updates agent config.
func (agent *AgentClient) UpdateAgentConfig(config *AgentConfig) error {
	configBuff, err := config.marshal()
	if err != nil {
		return err
	}

	url := agent.URL + agentConfigURL
	bodyString, err := handleRequest(http.MethodPut, url, configBuff)
	if err != nil {
		return fmt.Errorf("handleRequest error: %v", err)
	}

	logger.Debugf("update agent config: URL: %s request: %s result: %v",
		url, configBuff, string(bodyString))

	return err
}
