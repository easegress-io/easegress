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

package jmxtool

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	agentConfigURL = "/config"
	agentInfoURL   = "/agent-info"
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
		spec.Service `json:",inline"`

		Headers  string         `json:"easeagent.progress.forwarded.headers"`
		Reporter *AgentReporter `json:"reporter.outputServer"`
	}

	// AgentReporter is the basic config for agent reporter.
	AgentReporter struct {
		ReporterTLS *AgentReporterTLS `json:"tls"`

		AppendType      string `json:"appendType"`
		BootstrapServer string `json:"bootstrapServer"`
		Username        string `json:"username"`
		Password        string `json:"password"`
	}

	// AgentReporterTLS is the TLS config for agent resporter.
	AgentReporterTLS struct {
		Enable bool   `json:"enable"`
		Key    string `json:"key"`
		Cert   string `json:"cert"`
		CACert string `json:"ca_cert"`
	}

	// AgentInfo stores agent information.
	AgentInfo struct {
		Type    string `json:"type"`
		Version string `json:"version"`
	}
)

func (ac *AgentConfig) marshal() ([]byte, error) {
	jsonBuff, err := codectool.MarshalJSON(ac)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to json failed: %v", ac, err)
	}
	kvMap, err := JSONToKVMap(string(jsonBuff))
	if err != nil {
		return nil, fmt.Errorf("json to kv failed: %v", err)
	}

	result, err := codectool.MarshalJSON(kvMap)
	if err != nil {
		return nil, fmt.Errorf("marshal %s to json failed: %v", kvMap, err)
	}

	return result, nil
}

// NewAgentClient creates the agent client
func NewAgentClient(host, port string) *AgentClient {
	return &AgentClient{
		URL:        "http://" + host + ":" + port,
		HTTPClient: &http.Client{},
	}
}

// GetAgentInfo gets the information of the agent.
func (agent *AgentClient) GetAgentInfo() (*AgentInfo, error) {
	url := agent.URL + agentInfoURL
	bodyString, err := handleRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("handleRequest failed: %v", err)
	}

	info := &AgentInfo{}
	err = json.Unmarshal([]byte(bodyString), info)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s to json failed: %v", bodyString, err)
	}

	return info, nil
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
