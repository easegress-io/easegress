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

package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipeline(t *testing.T) {
	assert := assert.New(t)

	// fail to create Pipeline because of invalid yaml
	// invalid url
	yamlStr := `
name: pipeline-fail
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: 127.0.0.1:8888
`
	err := createResource(yamlStr)
	assert.Error(err)

	// success to create two Pipelines:
	// pipeline-success1 and pipeline-success2
	yamlStr = `
name: pipeline-success1
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:8888
`
	err = createResource(yamlStr)
	assert.NoError(err)

	yamlStr = `
name: pipeline-success2
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:8888
`
	err = createResource(yamlStr)
	assert.NoError(err)

	// list Pipeline and find them by using name
	output, err := getResource("pipeline", "-o", "yaml")
	assert.NoError(err)
	assert.True(strings.Contains(output, "name: pipeline-success1"))
	assert.True(strings.Contains(output, "name: pipeline-success2"))

	// update Pipeline and use list to find it
	yamlStr = `
name: pipeline-success2
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://update-pipeline-success2:8888
`
	err = applyResource(yamlStr)
	assert.NoError(err)

	// default get return a table now
	output, err = getResource("pipeline", "pipeline-success2")
	assert.NoError(err)
	assert.False(strings.Contains(output, "http://update-pipeline-success2:8888"))
	assert.True(strings.Contains(output, "pipeline-success2"))

	output, err = describeResource("pipeline", "pipeline-success2")
	assert.NoError(err)
	assert.True(strings.Contains(output, "http://update-pipeline-success2:8888"))
	assert.True(strings.Contains(output, "Name: pipeline-success2"))

	// delete all Pipelines
	err = deleteResource("pipeline", "pipeline-success1", "pipeline-success2")
	assert.NoError(err)

	output, err = getResource("pipeline")
	assert.NoError(err)
	assert.False(strings.Contains(output, "pipeline-success1"))
	assert.False(strings.Contains(output, "pipeline-success2"))
}

func TestHTTPServer(t *testing.T) {
	assert := assert.New(t)

	// fail to create HTTPServer because of invalid yaml
	yamlStr := `
name: httpserver-fail
kind: HTTPServer
`
	err := createResource(yamlStr)
	assert.Error(err)

	// success to create HTTPServer:
	// httpserver-success1
	yamlStr = `
name: httpserver-success
kind: HTTPServer
port: 10080
rules:
  - paths:
    - pathPrefix: /api
      backend: pipeline-api
`
	err = createResource(yamlStr)
	assert.NoError(err)

	// list HTTPServer and find it by name
	output, err := getResource("httpserver", "-o", "yaml")
	assert.NoError(err)
	assert.True(strings.Contains(output, "name: httpserver-success"))

	// update HTTPServer and use list to find it
	yamlStr = `
name: httpserver-success
kind: HTTPServer
port: 10080
rules:
  - paths:
    - pathPrefix: /api
      backend: update-httpserver-success
`
	err = applyResource(yamlStr)
	assert.NoError(err)

	output, err = describeResource("httpserver", "httpserver-success")
	assert.NoError(err)
	assert.True(strings.Contains(output, "backend: update-httpserver-success"))

	// delete all HTTPServer
	err = deleteResource("httpserver", "--all")
	assert.NoError(err)

	output, err = getResource("httpserver")
	assert.NoError(err)
	assert.False(strings.Contains(output, "httpserver-success"))
}

func TestHTTPServerAndPipeline(t *testing.T) {
	assert := assert.New(t)

	// create httpserver
	yamlStr := `
name: httpserver-test
kind: HTTPServer
port: 10081
https: false
keepAlive: true
keepAliveTimeout: 75s
maxConnection: 10240
cacheSize: 0
rules:
  - paths:
    - backend: pipeline-test
`
	err := createResource(yamlStr)
	assert.NoError(err)
	defer func() {
		err := deleteResource("httpserver", "httpserver-test")
		assert.NoError(err)
	}()

	// create pipeline
	yamlStr = `
name: pipeline-test
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:8888
`
	err = createResource(yamlStr)
	assert.NoError(err)
	defer func() {
		err := deleteResource("pipeline", "pipeline-test")
		assert.NoError(err)
	}()

	// create backend server with port 8888
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from backend")
	})
	server := startServer(8888, mux)
	defer server.Shutdown(context.Background())
	// check 8888 server is started
	started := checkServerStart(t, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8888", nil)
		require.Nil(t, err)
		return req
	})
	require.True(t, started)

	// send request to 10081 HTTPServer
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:10081/", nil)
	assert.Nil(err)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	assert.Equal("hello from backend", string(data))
}

func getMQTTClient(clientID, userName, password string) (paho.Client, error) {
	opts := paho.NewClientOptions().AddBroker("tcp://0.0.0.0:1883").SetClientID(clientID).SetUsername(userName).SetPassword(password)
	c := paho.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return c, nil
}

func TestMQTTProxy(t *testing.T) {
	assert := assert.New(t)

	yamlStr := `
kind: MQTTProxy
name: mqttproxy-test
port: 1883
`
	err := createResource(yamlStr)
	assert.NoError(err)
	defer func() {
		err := deleteResource("mqttproxy", "mqttproxy-test")
		assert.NoError(err)
	}()

	for i := 0; i < 10; i++ {
		_, err = getMQTTClient("client1", "test", "test")
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	assert.Nil(err)
}

func TestEgctlCmd(t *testing.T) {
	assert := assert.New(t)

	// health
	{
		cmd := egctlCmd("health")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.True(strings.Contains(output, "OK"))
	}

	// apis
	{
		cmd := egctlCmd("apis")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		head := []string{"PATH", "METHOD", "VERSION", "GROUP"}
		assert.True(matchTable(head, output))
		assert.True(matchTable([]string{"/apis/v2", "admin"}, output))
	}

	// api-resources
	{
		cmd := egctlCmd("api-resources")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		head := []string{"NAME", "ALIASES", "KIND", "ACTION"}
		assert.True(matchTable(head, output))
		assert.True(matchTable([]string{"member", "m,mem,members", "Member", "delete,get,describe"}, output))
		assert.Contains(output, "create,apply,delete,get,describe")
	}

	// completion
	{
		cmd := egctlCmd("completion", "bash")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.True(strings.Contains(output, "bash completion for egctl"))

		cmd = egctlCmd("completion", "zsh")
		output, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.True(strings.Contains(output, "zsh completion for egctl"))
	}

	// profile
	{
		ts := time.Now().Format("2006-01-02-1504")
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("egctl-test-%s", ts))
		assert.NoError(err)
		defer os.RemoveAll(tempDir)

		cpuPath := filepath.Join(tempDir, "cpu-profile")
		memoryPath := filepath.Join(tempDir, "memory-profile")

		cmd := egctlCmd("profile", "info")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.True(strings.Contains(output, `cpuPath: ""`))
		assert.True(strings.Contains(output, `memoryPath: ""`))

		cmd = egctlCmd("profile", "start", "cpu", cpuPath)
		_, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)

		cmd = egctlCmd("profile", "start", "memory", memoryPath)
		_, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)

		time.Sleep(1 * time.Second)

		cmd = egctlCmd("profile", "stop")
		_, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)

		cmd = egctlCmd("profile", "info")
		output, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, fmt.Sprintf("cpuPath: %s", cpuPath))
		assert.Contains(output, fmt.Sprintf("memoryPath: %s", memoryPath))
	}
}

func TestEgctlConfig(t *testing.T) {
	assert := assert.New(t)

	// reset home dir when finished
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		fmt.Println("HOME is empty, skip config test for this os")
		return
	}
	defer os.Setenv("HOME", homeDir)

	// set new home dir
	ts := time.Now().Format("2006-01-02-1504")
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("egctl-test-%s", ts))
	assert.NoError(err)
	defer os.RemoveAll(tempDir)
	os.Setenv("HOME", tempDir)

	egctlrc := `
clusters:
    - cluster:
        server: localhost:2381
      name: default
contexts:
    - context:
        cluster: default
        user: default
      name: default
    - context:
        cluster: default
        user: admin
      name: admin
current-context: default
kind: Config
users:
    - name: default
      user: {}
    - name: admin
      user:
        password: admin
        username: admin
`
	err = os.WriteFile(filepath.Join(tempDir, ".egctlrc"), []byte(egctlrc), os.ModePerm)
	assert.NoError(err)

	cmd := egctlCmd("config", "view")
	output, stderr, err := runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.True(strings.Contains(output, "kind: Config"))
	assert.True(strings.Contains(output, "server: localhost:2381"))

	cmd = egctlCmd("config", "current-context")
	output, stderr, err = runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.True(strings.Contains(output, "name: default"))
	assert.True(strings.Contains(output, "user: default"))
	assert.True(strings.Contains(output, "cluster: default"))

	cmd = egctlCmd("config", "get-contexts")
	output, stderr, err = runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.True(matchTable([]string{"CURRENT", "NAME", "CLUSTER", "USER"}, output))
	assert.True(matchTable([]string{"default", "default", "default"}, output))
	assert.True(matchTable([]string{"admin", "default", "admin"}, output))

	cmd = egctlCmd("config", "use-context", "admin")
	output, stderr, err = runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.True(strings.Contains(output, "Switched to context admin"))
}

func TestMatch(t *testing.T) {
	assert := assert.New(t)

	assert.True(matchTable([]string{"a", "b", "c"}, "a\t\tb\t\t\t\tc"))
	assert.False(matchTable([]string{"a", "b", "c", "d"}, "a\t\tb\t\t\t\tc"))
	assert.False(matchTable([]string{"a", "d"}, "a\t\tb"))
}

func TestMember(t *testing.T) {
	assert := assert.New(t)

	output, err := getResource("member")
	assert.NoError(err)
	assert.True(matchTable([]string{"NAME", "ROLE", "AGE", "STATE", "API-ADDR", "HEARTBEAT"}, output))
	assert.Contains(output, "primary")
	assert.Contains(output, "Leader")

	output, err = describeResource("mem")
	assert.NoError(err)
	assert.True(strings.Contains(output, "Name: primary-single"))
	assert.True(strings.Contains(output, "ClusterRole: primary"))
}

func TestCustomData(t *testing.T) {
	assert := assert.New(t)

	cdkYaml := `
name: custom-data-kind1
kind: CustomDataKind
idField: name
`
	cdYaml := `
rebuild: true
name: custom-data-kind1
kind: CustomData
list:
- name: data1
  field1: 12
- name: data2
  field1: foo
`
	err := createResource(cdkYaml)
	assert.NoError(err)

	err = applyResource(cdYaml)
	assert.NoError(err)

	output, err := getResource("cdk")
	assert.NoError(err)
	assert.True(matchTable([]string{"NAME", "ID-FIELD", "JSON-SCHEMA", "DATA-NUM"}, output))
	assert.True(matchTable([]string{"custom-data-kind1", "name", "no", "2"}, output))

	output, err = getResource("cd", "custom-data-kind1")
	assert.NoError(err)
	assert.Contains(output, "NAME")
	assert.Contains(output, "data1")
	assert.Contains(output, "data2")

	output, err = describeResource("cdk")
	assert.NoError(err)
	assert.Contains(output, "Name: custom-data-kind1")
	assert.Contains(output, "IdField: name")

	output, err = describeResource("cd", "custom-data-kind1", "data1")
	assert.NoError(err)
	assert.Contains(output, "Name: data1")
	assert.Contains(output, "Field1: 12")

	err = deleteResource("cd", "custom-data-kind1", "data1")
	assert.NoError(err)

	output, err = describeResource("cd", "custom-data-kind1")
	assert.NoError(err)
	assert.NotContains(output, "Name: data1")
	assert.NotContains(output, "Field1: 12")

	err = deleteResource("cdk", "custom-data-kind1")
	assert.NoError(err)
}
