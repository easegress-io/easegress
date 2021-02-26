package jmx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type JMXRequestOperationType string

const (
	JmxMbeanAttributeRead  JMXRequestOperationType = "read"
	JmxMbeanAttributeWrite JMXRequestOperationType = "write"
	JmxMbeanList           JMXRequestOperationType = "list"
	JmxMbeanSearch         JMXRequestOperationType = "search"
	JmxMbeanExec           JMXRequestOperationType = "exec"
)

type JolokiaInterface interface {
	GetMbeanAttribute(mbean string, attribute string, path string) (interface{}, error)
	SetMbeanAttribute(mbean string, attribute string, path string, value string) (interface{}, error)
	ListMbean(mbean string) (interface{}, error)
	SearchMbeans(pattern string) (interface{}, error)
	ExecuteMbeanOperation(mbean string, operation string, arguments []string) (interface{}, error)
}

type (
	JolokiaResponse struct {
		Status    uint32
		Timestamp uint32
		Request   map[string]interface{}
		Value     interface{}
		Error     string
	}

	readBody struct {
		Type      JMXRequestOperationType `json:"type"`
		Mbean     string                  `json:"mbean"`
		Attribute string                  `json:"attribute"`
		Path      string                  `json:"path"`
	}

	JolokiaClient struct {
		URL        string
		HttpClient *http.Client
	}
)

func NewClient(host, port, path string) *JolokiaClient {
	return &JolokiaClient{
		"http://" + host + ":" + port + "/" + path + "/",
		&http.Client{},
	}
}

func (c *JolokiaClient) handleRequest(requestBody []byte) (*JolokiaResponse, error) {

	request, err := http.NewRequest(
		"POST",
		c.URL,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.HttpClient.Do(request)
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
		return nil, fmt.Errorf("status:%d", jolokiaResponse.Status)
	}

	return &jolokiaResponse, nil

	//fmt.Printf(" == %s\n", requestBody)
	//request, err := http.NewRequest(
	//	"POST",
	//	JMXAgentURL,
	//	bytes.NewBuffer(requestBody),
	//)
	//if err != nil {
	//	return nil, err
	//}
	//request.Header.Set("Content-Type", "application/json")
	//httpClient := &http.Client{Timeout: time.Duration(10 * time.Second)}
	//response, err := httpClient.Do(request)
	//if err != nil {
	//	return nil, err
	//}
	//defer response.Body.Close()
	//contents, err := ioutil.ReadAll(response.Body)
	//if err != nil {
	//	return nil, err
	//}
	//
	//jolokiaReadResponse := JolokiaResponse{}
	//err = json.Unmarshal(contents, &response)
	//fmt.Println(jolokiaReadResponse)
	//fmt.Println(jolokiaReadResponse.Value)
	//if err != nil {
	//	return nil, err
	//}
	//if jolokiaReadResponse.Status != 200 {
	//	return nil, fmt.Errorf("status: %d", jolokiaReadResponse.Status)
	//}
	//
	//return &jolokiaReadResponse, nil

}

func (c *JolokiaClient) GetMbeanAttribute(bean, attr, path string, ) (interface{}, error) {
	//requestBody := make(map[string]string)
	request := readBody{
		Mbean:     bean,
		Attribute: attr,
		Type:      JmxMbeanAttributeRead,
	}
	if path != "" {
		request.Path = path
	}

	var err error
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := c.handleRequest(body)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

func (client JolokiaClient) SetMbeanAttribute(mbean string, attribute string, path string, value string) (interface{}, error) {
	return nil, nil
}

func (client *JolokiaClient) ListMbean(mbean string) (interface{}, error) {
	return nil, nil
}

func (client *JolokiaClient) SearchMbeans(pattern string) (interface{}, error) {
	return nil, nil
}

func (client *JolokiaClient) ExecuteMbeanOperation(mbean string, operation string, arguments []string) (interface{}, error) {
	return nil, nil
}
