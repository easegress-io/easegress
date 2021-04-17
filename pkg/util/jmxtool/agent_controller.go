package jmxtool

import (
	"encoding/json"
	"fmt"
	yamljsontool "github.com/ghodss/yaml"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"gopkg.in/yaml.v2"
	"net/http"
	"strconv"
)

const (
	canaryConfigURL  = " /canary-config"
	serviceConfigURL = " /service-config"
)

type AgentInterface interface {
	UpdateService(newService *spec.Service, version int64) error
	UpdateCanary(globalHeaders *spec.GlobalCanaryHeaders, version int64) error
}

type AgentClient struct {
	URL        string
	HttpClient *http.Client
}

func NewAgentClient(host, port, path string) *AgentClient {
	return &AgentClient{
		"http://" + host + ":" + port + "/" + path,
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
	kvMap, err := JsonToKVMap(string(jsonBytes))
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
	kvMap, err := JsonToKVMap(string(jsonBytes))
	kvMap["version"] = strconv.FormatInt(version, 10)

	bytes, err := json.Marshal(kvMap)
	if err != nil {
		return fmt.Errorf("marshal %s to json failed: %v", kvMap, err)
	}

	url := agent.URL + canaryConfigURL
	resp, err := handleRequest(http.MethodPut, url, bytes)
	logger.Infof("Update Canary, URL: %s, request %s, result: %v", string(bytes), resp)
	return err
}
