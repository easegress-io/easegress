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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type JMXRequestOperationType string

const (
	jmxMbeanAttributeRead  JMXRequestOperationType = "read"
	jmxMbeanAttributeWrite JMXRequestOperationType = "write"
	jmxMbeanExec           JMXRequestOperationType = "exec"
	jmxMbeanList           JMXRequestOperationType = "list"
	jmxMbeanSearch         JMXRequestOperationType = "search"
)

type JXMInterface interface {
	GetMbeanAttribute(mbean string, attribute string, path string) (interface{}, error)
	SetMbeanAttribute(mbean string, attribute string, path string, value interface{}) (interface{}, error)
	ExecuteMbeanOperation(mbean string, operation string, arguments []interface{}) (interface{}, error)
	ListMbean(mbean string) (interface{}, error)
	SearchMbeans(pattern string) (interface{}, error)
}

type (
	JolokiaResponse struct {
		Status    uint32
		Timestamp uint32
		Request   map[string]interface{}
		Value     interface{}
		Error     string
	}

	readRequestBody struct {
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
		Attribute string                  `json:"attribute"`
		Path      string                  `json:"path"`
	}

	writeRequestBody struct {
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
		Attribute string                  `json:"attribute"`
		Path      string                  `json:"path"`
		Value     interface{}             `json:"value"`
	}

	executeRequestBody struct {
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
		Operation string                  `json:"operation"`
		Arguments []interface{}           `json:"arguments"`
	}

	listRequestBody struct {
		Type  JMXRequestOperationType `json:"type"`
		Mbean string                  `json:"mbean"`
	}

	searchRequestBody struct {
		Type  JMXRequestOperationType `json:"type"`
		Mbean string                  `json:"mbean"`
	}

	JolokiaClient struct {
		URL        string
		HTTPClient *http.Client
	}
)

func NewJolokiaClient(host, port, path string) *JolokiaClient {
	return &JolokiaClient{
		"http://" + host + ":" + port + "/" + path + "/",
		&http.Client{},
	}
}

func (client *JolokiaClient) handleRequest(requestBody []byte) (*JolokiaResponse, error) {
	request, err := http.NewRequest(
		"POST",
		client.URL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	jolokiaResponse := JolokiaResponse{}
	err = json.Unmarshal(contents, &jolokiaResponse)
	if err != nil {
		return nil, err
	}
	if jolokiaResponse.Status != 200 {
		return nil, fmt.Errorf("status: %d %s", jolokiaResponse.Status, jolokiaResponse.Error)
	}

	return &jolokiaResponse, nil
}

func (client *JolokiaClient) execute(requestBody interface{}) (interface{}, error) {
	var err error
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.handleRequest(body)
	if err != nil {
		return nil, fmt.Errorf("access jmx failed: %v", err)
	}
	return resp.Value, err
}

func (client *JolokiaClient) GetMbeanAttribute(mbean string, attribute string, path string) (interface{}, error) {
	requestBody := readRequestBody{
		Mbean:     mbean,
		Attribute: attribute,
		Type:      jmxMbeanAttributeRead,
	}
	if path != "" {
		requestBody.Path = path
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read Mbean Attribute failed: %v", err)
	}
	return result, err
}

func (client *JolokiaClient) SetMbeanAttribute(mbean string, attribute string, path string, value interface{}) (interface{}, error) {
	requestBody := writeRequestBody{
		Mbean:     mbean,
		Attribute: attribute,
		Type:      jmxMbeanAttributeWrite,
		Value:     value,
	}
	if path != "" {
		requestBody.Path = path
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("write Mbean Attribute failed: %v", err)
	}
	return result, nil
}

func (client *JolokiaClient) ExecuteMbeanOperation(mbean string, operation string, arguments []interface{}) (interface{}, error) {
	requestBody := executeRequestBody{
		Mbean:     mbean,
		Operation: operation,
		Type:      jmxMbeanExec,
		Arguments: arguments,
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("execute Mbean Operation failed: %v", err)
	}
	return result, nil
}

func (client *JolokiaClient) ListMbean(mbean string) (interface{}, error) {
	requestBody := listRequestBody{
		Type:  jmxMbeanList,
		Mbean: mbean,
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("list Mbean failed: %v", err)
	}
	return result, nil
}

func (client *JolokiaClient) SearchMbeans(pattern string) (interface{}, error) {
	requestBody := searchRequestBody{
		Type:  jmxMbeanSearch,
		Mbean: pattern,
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("search Mbean failed: %v", err)
	}
	return result, nil
}
