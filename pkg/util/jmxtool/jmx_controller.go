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
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
	}

	searchRequestBody struct {
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
	}

	JolokiaClient struct {
		URL        string
		HttpClient *http.Client
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
	response, err := client.HttpClient.Do(request)
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
		return nil, fmt.Errorf("status:%d, %s", jolokiaResponse.Status, jolokiaResponse.Error)
	}

	return &jolokiaResponse, nil

}

func (client *JolokiaClient) execute(requestBody interface{})(interface{}, error) {
	var err error
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.handleRequest(body)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("Read Mbean Attribute faield:  %v", err)
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
		return nil, fmt.Errorf("Write Mbean Attribute faield:  %v", err)
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
		return nil, fmt.Errorf("Executee Mbean Operation faield:  %v", err)
	}
	return result, nil
}


func (client *JolokiaClient) ListMbean(mbean string) (interface{}, error) {
	requestBody := listRequestBody{
		Type:      jmxMbeanList,
		Mbean:     mbean,
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("List Mbean faield:  %v", err)
	}
	return result, nil
}


func (client *JolokiaClient) SearchMbeans(pattern string) (interface{}, error) {
	requestBody := searchRequestBody{
		Type:      jmxMbeanSearch,
		Mbean:     pattern,
	}

	result, err := client.execute(requestBody)
	if err != nil {
		return nil, fmt.Errorf("Search Mbean faield:  %v", err)
	}
	return result, nil
}
