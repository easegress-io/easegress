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

package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
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
	assert.Contains(output, "name: pipeline-success1")
	assert.Contains(output, "name: pipeline-success2")

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
	assert.NotContains(output, "http://update-pipeline-success2:8888")
	assert.Contains(output, "pipeline-success2")

	output, err = describeResource("pipeline", "pipeline-success2")
	assert.NoError(err)
	assert.Contains(output, "http://update-pipeline-success2:8888")
	assert.Contains(output, "Name: pipeline-success2")

	// delete all Pipelines
	err = deleteResource("pipeline", "pipeline-success1", "pipeline-success2")
	assert.NoError(err)

	output, err = getResource("pipeline")
	assert.NoError(err)
	assert.NotContains(output, "pipeline-success1")
	assert.NotContains(output, "pipeline-success2")
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
	assert.Contains(output, "name: httpserver-success")

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
	assert.Contains(output, "backend: update-httpserver-success")

	// delete all HTTPServer
	err = deleteResource("httpserver", "--all")
	assert.NoError(err)

	output, err = getResource("httpserver")
	assert.NoError(err)
	assert.NotContains(output, "httpserver-success")
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
	server := mustStartServer(8888, mux, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8888", nil)
		require.Nil(t, err)
		return req
	})
	defer server.Shutdown(context.Background())

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
		assert.Contains(output, "OK")
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
		head := []string{"NAME", "ALIASES", "CATEGORY", "KIND", "ACTION"}
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
		assert.Contains(output, "bash completion for egctl")

		cmd = egctlCmd("completion", "zsh")
		output, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "zsh completion for egctl")
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
		assert.Contains(output, `cpuPath: ""`)
		assert.Contains(output, `memoryPath: ""`)

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
	assert.Contains(output, "kind: Config")
	assert.Contains(output, "server: localhost:2381")

	cmd = egctlCmd("config", "current-context")
	output, stderr, err = runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	assert.Contains(output, "name: default")
	assert.Contains(output, "user: default")
	assert.Contains(output, "cluster: default")

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
	assert.Contains(output, "Switched to context admin")
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
	assert.Contains(output, "Name: primary-single")
	assert.Contains(output, "ClusterRole: primary")
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

func TestCreateHTTPProxy(t *testing.T) {
	assert := assert.New(t)

	servers := make([]*http.Server, 0)
	for _, port := range []int{9096, 9097, 9098} {
		currentPort := port
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "hello from backend %d", currentPort)
		})
		server := mustStartServer(currentPort, mux, func() *http.Request {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d", currentPort), nil)
			require.Nil(t, err)
			return req
		})
		servers = append(servers, server)
	}
	defer func() {
		for _, s := range servers {
			s.Shutdown(context.Background())
		}
	}()

	cmd := egctlCmd(
		"create",
		"httpproxy",
		"http-proxy-test",
		"--port", "10080",
		"--rule",
		"/pipeline=http://127.0.0.1:9096",
		"--rule",
		"/barz=http://127.0.0.1:9097",
		"--rule",
		"/bar*=http://127.0.0.1:9098",
	)
	_, stderr, err := runCmd(cmd)
	assert.NoError(err)
	assert.Empty(stderr)
	defer func() {
		deleteResource("httpserver", "http-proxy-test")
		deleteResource("pipeline", "--all")
	}()

	output, err := getResource("httpserver")
	assert.NoError(err)
	assert.Contains(output, "http-proxy-test")

	output, err = getResource("pipeline")
	assert.NoError(err)
	assert.Contains(output, "http-proxy-test-0")

	testFn := func(p string, expected string) {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:10080"+p, nil)
		assert.Nil(err, p)
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err, p)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		assert.Nil(err, p)
		assert.Equal(expected, string(data), p)
	}

	testFn("/pipeline", "hello from backend 9096")
	testFn("/barz", "hello from backend 9097")
	testFn("/bar-prefix", "hello from backend 9098")
}

func TestLogs(t *testing.T) {
	assert := assert.New(t)

	{
		// test egctl logs --tail n
		cmd := egctlCmd("logs", "--tail", "5")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "INFO")
		assert.Contains(output, ".go", "should contain go file in log")
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(lines, 5)
	}
	{
		// test egctl logs -f
		cmd := egctlCmd("logs", "-f", "--tail", "0")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		assert.NoError(err)

		// check if new logs are printed
		yamlStr := `
kind: HTTPServer
name: test-egctl-logs
port: 12345
rules:
- paths:
  - pathPrefix: /pipeline
    backend: pipeline-demo
`
		err = applyResource(yamlStr)
		assert.NoError(err)
		time.Sleep(1 * time.Second)
		cmd.Process.Kill()
		assert.Contains(stdout.String(), "test-egctl-logs")
		deleteResource("httpserver", "test-egctl-logs")
	}

	{
		cmd := egctlCmd("logs", "get-level")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "info")

		cmd = egctlCmd("logs", "set-level", "debug")
		output, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "debug")

		cmd = egctlCmd("logs", "get-level")
		output, stderr, err = runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "debug")
	}
}

func TestMetrics(t *testing.T) {
	assert := assert.New(t)
	{
		cmd := egctlCmd("metrics")
		output, stderr, err := runCmd(cmd)
		assert.NoError(err)
		assert.Empty(stderr)
		assert.Contains(output, "etcd_server_has_leader")
	}
}

func TestHealthCheck(t *testing.T) {
	assert := assert.New(t)

	// unhealthy server return 503 as status code
	var invalidCode int32 = 503
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Server", "unhealthy")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		code := atomic.LoadInt32(&invalidCode)
		w.WriteHeader(int(code))
	})
	unhealthy := mustStartServer(12345, mux, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:12345", nil)
		require.Nil(t, err)
		return req
	})
	defer unhealthy.Shutdown(context.Background())

	// healthy server return 200 as status code
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Server", "healthy")
		w.WriteHeader(http.StatusOK)
	})
	mux2.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthy := mustStartServer(12346, mux2, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:12346", nil)
		require.Nil(t, err)
		return req
	})
	defer healthy.Shutdown(context.Background())

	httpSeverYaml := `
name: httpserver-hc
kind: HTTPServer
port: 9099
rules:
- paths:
  - pathPrefix: /
    backend: pipeline-hc
`
	pipelineYaml := `
name: pipeline-hc
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:12345
    - url: http://127.0.0.1:12346
    loadBalance:
      policy: roundRobin
      healthCheck:
        interval: 200ms
        fails: 2
        pass: 2
        path: /health
`
	err := createResource(httpSeverYaml)
	assert.Nil(err)
	defer deleteResource("httpserver", "httpserver-hc")
	err = createResource(pipelineYaml)
	defer deleteResource("pipeline", "pipeline-hc")
	assert.Nil(err)
	started := checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9099", nil)
		assert.Nil(err)
		return req
	})
	assert.True(started)

	time.Sleep(1 * time.Second)
	// unhealthy server not passed health check.
	for i := 0; i < 50; i++ {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9099", nil)
		assert.Nil(err, i)
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err, i)
		assert.Equal(http.StatusOK, resp.StatusCode, i)
		assert.Equal("healthy", resp.Header.Get("X-Server"), i)
		resp.Body.Close()
	}

	atomic.StoreInt32(&invalidCode, 200)
	time.Sleep(1 * time.Second)
	last := ""
	// unhealthy server passed health check.
	// based on round robin, the response should be unhealthy, healthy, unhealthy, healthy...
	for i := 0; i < 50; i++ {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9099", nil)
		assert.Nil(err, i)
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err, i)
		assert.Equal(http.StatusOK, resp.StatusCode, i)
		value := resp.Header.Get("X-Server")
		if last != "" {
			if last == "healthy" {
				assert.Equal("unhealthy", value, i)
			} else {
				assert.Equal("healthy", value, i)
			}
		}
		last = value
		resp.Body.Close()
	}
}

func TestHealthCheck2(t *testing.T) {
	assert := assert.New(t)

	httpSeverYaml := `
name: httpserver-hc
kind: HTTPServer
port: 9099
rules:
- paths:
  - pathPrefix: /
    backend: pipeline-hc
`
	pipelineYaml := `
name: pipeline-hc
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:12345
    healthCheck:
      interval: 200ms
      fails: 2
      pass: 2
      uri: /health?proxy=easegress
      method: POST
      headers:
        X-Health: easegress
      body: "easegress"
      username: admin
      password: test-health-check
      match:
        statusCodes:
        - [200, 399]
        headers:
        - name: X-Status
          value: healthy
        body:
          value: "healthy"
`
	// check if health check set request correctly
	requestChecker := func(req *http.Request) error {
		if req.URL.Path != "/health" || req.URL.Query().Get("proxy") != "easegress" {
			return fmt.Errorf("invalid request url: %s", req.URL.String())
		}
		if req.Method != http.MethodPost {
			return fmt.Errorf("invalid request method: %s", req.Method)
		}
		if req.Header.Get("X-Health") != "easegress" {
			return fmt.Errorf("invalid request header: %s", req.Header.Get("X-Health"))
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if string(body) != "easegress" {
			return fmt.Errorf("invalid request body: %s", string(body))
		}
		username, password, ok := req.BasicAuth()
		if !ok {
			return fmt.Errorf("failed to get basic auth")
		}
		if username != "admin" || password != "test-health-check" {
			return fmt.Errorf("invalid basic auth: %s:%s", username, password)
		}
		return nil
	}

	healthCheckHandler := atomic.Value{}
	healthCheckHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Status", "healthy")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})
	callHealthCheck := atomic.Bool{}
	callHealthCheck.Store(false)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Server", "server")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		callHealthCheck.Store(true)
		err := requestChecker(r)
		assert.Nil(err)
		handler := healthCheckHandler.Load().(func(w http.ResponseWriter, r *http.Request))
		handler(w, r)
	})
	server := mustStartServer(12345, mux, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:12345", nil)
		require.Nil(t, err)
		return req
	})
	defer server.Shutdown(context.Background())

	err := createResource(httpSeverYaml)
	assert.Nil(err)
	defer deleteResource("httpserver", "httpserver-hc")
	err = createResource(pipelineYaml)
	defer deleteResource("pipeline", "pipeline-hc")
	assert.Nil(err)
	started := checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9099", nil)
		assert.Nil(err)
		return req
	})
	assert.True(started)

	doReq := func() *http.Response {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9099", nil)
		assert.Nil(err)
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err)
		return resp
	}

	// health check passed
	time.Sleep(1 * time.Second)
	resp := doReq()
	resp.Body.Close()
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("server", resp.Header.Get("X-Server"))

	// health check failed, wrong status code
	healthCheckHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Status", "healthy")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("healthy"))
	})
	time.Sleep(1 * time.Second)
	resp = doReq()
	resp.Body.Close()
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	// health check failed, wrong body
	healthCheckHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Status", "healthy")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not ok"))
	})
	time.Sleep(1 * time.Second)
	resp = doReq()
	resp.Body.Close()
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	// health check failed, wrong header
	healthCheckHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Status", "unhealthy")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})
	time.Sleep(800 * time.Millisecond)
	resp = doReq()
	resp.Body.Close()
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	// health check passed
	healthCheckHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Status", "healthy")
		w.WriteHeader(http.StatusFound)
		w.Write([]byte("very healthy"))
	})
	time.Sleep(1 * time.Second)
	resp = doReq()
	resp.Body.Close()
	assert.Equal(http.StatusOK, resp.StatusCode)
	assert.Equal("server", resp.Header.Get("X-Server"))

	called := callHealthCheck.Load()
	assert.True(called)
}

func TestWebSocketHealthCheck(t *testing.T) {
	assert := assert.New(t)

	httpSeverYaml := `
name: httpserver-hc
kind: HTTPServer
port: 9099
rules:
- paths:
  - headers:
    - key: Upgrade
      values:
      - websocket
    backend: pipeline-ws
    clientMaxBodySize: -1
`
	wsYaml := `
name: pipeline-ws
kind: Pipeline
filters:
- name: websocket
  kind: WebSocketProxy
  pools:
  - servers:
    - url: ws://127.0.0.1:12345
    healthCheck:
      interval: 200ms
      timeout: 200ms
      http:
        uri: /health
      ws:
        uri: /ws
`

	upgrader := &websocket.Upgrader{}

	httpHandler := atomic.Value{}
	httpHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wsHandler := atomic.Value{}
	wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		assert.Nil(err)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// for websocket proxy filter to access
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.Nil(err)
		defer conn.Close()
		conn.WriteMessage(websocket.TextMessage, []byte("hello from websocket"))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// for http health check of websocket proxy filter
		handler := httpHandler.Load().(func(w http.ResponseWriter, r *http.Request))
		handler(w, r)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// for ws health check of websocket proxy filter
		handler := wsHandler.Load().(func(w http.ResponseWriter, r *http.Request))
		handler(w, r)
	})
	server := mustStartServer(12345, mux, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:12345/health", nil)
		require.Nil(t, err)
		return req
	})
	defer server.Shutdown(context.Background())

	err := createResource(httpSeverYaml)
	assert.Nil(err)
	defer deleteResource("httpserver", "httpserver-hc")
	err = createResource(wsYaml)
	assert.Nil(err)
	defer deleteResource("pipeline", "pipeline-ws")
	time.Sleep(1 * time.Second)

	// health check passed
	time.Sleep(1 * time.Second)
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9099", nil)
	assert.Nil(err)
	_, data, err := conn.ReadMessage()
	assert.Nil(err)
	assert.Equal("hello from websocket", string(data))
	conn.Close()

	// health check failed
	wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	time.Sleep(1 * time.Second)
	_, resp, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:9099", nil)
	assert.NotNil(err)
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	// health check passed again
	wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
		_, err := upgrader.Upgrade(w, r, nil)
		assert.Nil(err)
	})
	time.Sleep(1 * time.Second)
	conn, resp, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:9099", nil)
	assert.Nil(err)
	assert.Equal(http.StatusSwitchingProtocols, resp.StatusCode)
	_, data, err = conn.ReadMessage()
	assert.Nil(err)
	assert.Equal("hello from websocket", string(data))
}

func TestEgbuilder(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := os.MkdirTemp("", "easegress-test")
	require.Nil(t, err)
	defer os.RemoveAll(tempDir)

	// init a new plugin repo
	initCmd := egbuilderCmd(
		"init",
		"--repo", "github.com/test/repo",
		"--filters=MyFilter1",
		"--controllers=MyController1,MyController2",
	)
	initCmd.Dir = tempDir
	stdout, stderr, err := runCmd(initCmd)
	fmt.Printf("init stdout:\n%s\n", stdout)
	fmt.Printf("init stderr:\n%s\n", stderr)
	assert.NoError(err)

	// add a new filters and controllers
	addCmd := egbuilderCmd(
		"add",
		"--filters=MyFilter2",
		"--controllers=MyController3",
	)
	addCmd.Dir = tempDir
	stdout, stderr, err = runCmd(addCmd)
	fmt.Printf("add stdout:\n%s\n", stdout)
	fmt.Printf("add stderr:\n%s\n", stderr)
	assert.NoError(err)

	// build easegress with new plugins
	buildConfig := `
plugins:
- module: github.com/test/repo
  version: ""
  replacement: %s

output: "%s/easegress-server"
`
	buildConfig = fmt.Sprintf(buildConfig, tempDir, tempDir)
	err = os.WriteFile(filepath.Join(tempDir, "build.yaml"), []byte(buildConfig), os.ModePerm)
	assert.NoError(err)

	buildCmd := egbuilderCmd(
		"build",
		"-f",
		"build.yaml",
	)
	buildCmd.Dir = tempDir
	stdout, stderr, err = runCmd(buildCmd)
	fmt.Printf("build stdout:\n%s\n", stdout)
	fmt.Printf("build stderr:\n%s\n", stderr)
	assert.NoError(err)

	// run easegress with new plugins
	egserverConfig := `
name: egbuilder
cluster-name: egbuilder-test
cluster-role: primary
cluster:
  listen-peer-urls:
   - http://localhost:22380
  listen-client-urls:
   - http://localhost:22379
  advertise-client-urls:
   - http://localhost:22379
  initial-advertise-peer-urls:
   - http://localhost:22380
  initial-cluster:
   - egbuilder: http://localhost:22380
api-addr: 127.0.0.1:22381
`
	apiURL := "http://127.0.0.1:22381"
	err = os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(egserverConfig), os.ModePerm)
	assert.Nil(err)

	runEgCmd := exec.Command(
		filepath.Join(tempDir, "easegress-server"),
		"--config-file",
		"config.yaml",
	)
	runEgCmd.Dir = tempDir
	var stdoutBuf, stderrBuf bytes.Buffer
	runEgCmd.Stdout = &stdoutBuf
	runEgCmd.Stderr = &stderrBuf
	err = runEgCmd.Start()
	assert.Nil(err)
	defer func() {
		err := runEgCmd.Process.Signal(os.Interrupt)
		assert.Nil(err)
		err = runEgCmd.Wait()
		assert.Nil(err)
		assert.NotContains(stderrBuf.String(), "panic")
		assert.NotContains(stdoutBuf.String(), "panic")
	}()

	started := checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, apiURL+"/apis/v2/healthz", nil)
		assert.Nil(err)
		return req
	})
	assert.True(started)

	egctl := func(args ...string) *exec.Cmd {
		return egctlWithServer(apiURL, args...)
	}

	// create, apply, delete new controllers
	controllers := `
name: c1
kind: MyController1

---

name: c2
kind: MyController2

---

name: c3
kind: MyController3
`
	controllerNames := []string{"c1", "c2", "c3"}
	controllerKinds := []string{"MyController1", "MyController2", "MyController3"}

	cmd := egctl("create", "-f", "-")
	cmd.Stdin = strings.NewReader(controllers)
	stdout, stderr, err = runCmd(cmd)
	fmt.Printf("egctl create stdout:\n%s\n", stdout)
	assert.Contains(stdout, "create MyController1 c1 successfully")
	assert.Contains(stdout, "create MyController2 c2 successfully")
	assert.Contains(stdout, "create MyController3 c3 successfully")
	assert.Empty(stderr)
	assert.NoError(err)

	cmd = egctl("get", "all")
	stdout, stderr, err = runCmd(cmd)
	for i := range controllerNames {
		assert.Contains(stdout, controllerNames[i])
		assert.Contains(stdout, controllerKinds[i])
	}
	assert.Empty(stderr)
	assert.NoError(err)

	cmd = egctl("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(controllers)
	stdout, stderr, err = runCmd(cmd)
	fmt.Printf("egctl apply stdout:\n%s\n", stdout)
	assert.Contains(stdout, "update MyController1 c1 successfully")
	assert.Contains(stdout, "update MyController2 c2 successfully")
	assert.Contains(stdout, "update MyController3 c3 successfully")
	assert.Empty(stderr)
	assert.NoError(err)

	for i := range controllerNames {
		cmd = egctl("delete", controllerKinds[i], controllerNames[i])
		stdout, stderr, err = runCmd(cmd)
		fmt.Printf("egctl delete stdout:\n%s\n", stdout)
		assert.Contains(stdout, fmt.Sprintf("delete %s %s successfully", controllerKinds[i], controllerNames[i]))
		assert.Empty(stderr)
		assert.NoError(err)
	}

	cmd = egctl("get", "all")
	stdout, stderr, err = runCmd(cmd)
	for i := range controllerNames {
		assert.NotContains(stdout, controllerNames[i])
		assert.NotContains(stdout, controllerKinds[i])
	}
	assert.Empty(stderr)
	assert.NoError(err)

	// create, apply, delete new filters
	filters := `
name: httpserver
kind: HTTPServer
port: 22399
rules:
- paths:
  - backend: pipeline

---

name: pipeline
kind: Pipeline
flow:
- filter: filter1
- filter: filter2
- filter: mock
filters:
- name: filter1
  kind: MyFilter1
- name: filter2
  kind: MyFilter2
- name: mock
  kind: ResponseBuilder
  template: |
    statusCode: 200
    body: "body from response builder"
`

	cmd = egctl("create", "-f", "-")
	cmd.Stdin = strings.NewReader(filters)
	stdout, stderr, err = runCmd(cmd)
	fmt.Printf("egctl create stdout:\n%s\n", stdout)
	assert.Empty(stderr)
	assert.NoError(err)

	cmd = egctl("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(filters)
	stdout, stderr, err = runCmd(cmd)
	fmt.Printf("egctl apply stdout:\n%s\n", stdout)
	assert.Empty(stderr)
	assert.NoError(err)

	started = checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:22399", nil)
		assert.Nil(err)
		return req
	})
	assert.True(started)

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:22399", nil)
	assert.Nil(err)
	resp, err := http.DefaultClient.Do(req)
	assert.Nil(err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	assert.Nil(err)
	// get this body means our pipeline is working.
	assert.Equal("body from response builder", string(data))
}

func TestEgctlNamespace(t *testing.T) {
	assert := assert.New(t)
	mockNamespace := `
name: mockNamespace
kind: MockNamespacer
namespace: mockNamespace
httpservers:
- kind: HTTPServer
  name: mock-httpserver
  port: 10222
  https: false
  rules:
    - paths:
      - path: /pipeline
        backend: mock-pipeline
pipelines:
- name: mock-pipeline
  kind: Pipeline
  flow:
    - filter: proxy
  filters:
    - name: proxy
      kind: Proxy
      pools:
      - servers:
        - url: http://127.0.0.1:9095
        - url: http://127.0.0.1:9096
        loadBalance:
          policy: roundRobin
`
	err := createResource(mockNamespace)
	assert.Nil(err)
	defer func() {
		err := deleteResource("MockNamespacer", "mockNamespace")
		assert.Nil(err)
	}()

	httpserver := `
name: httpserver-test
kind: HTTPServer
port: 10181
https: false
keepAlive: true
keepAliveTimeout: 75s
maxConnection: 10240
cacheSize: 0
rules:
  - paths:
    - backend: pipeline-test
`
	err = createResource(httpserver)
	assert.Nil(err)
	defer func() {
		err := deleteResource("HTTPServer", "httpserver-test")
		assert.Nil(err)
	}()

	pipeline := `
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
	err = createResource(pipeline)
	assert.Nil(err)
	defer func() {
		err := deleteResource("Pipeline", "pipeline-test")
		assert.Nil(err)
	}()

	// egctl get all
	{
		// by default, list resources in "default" namespace
		cmd := egctlCmd("get", "all")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "httpserver-test")
		assert.Contains(output, "pipeline-test")
		assert.NotContains(output, "mock-httpserver")
		assert.NotContains(output, "mock-pipeline")
	}

	// egctl get all --all-namespaces
	{
		cmd := egctlCmd("get", "all", "--all-namespaces")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "httpserver-test")
		assert.Contains(output, "pipeline-test")
		assert.Contains(output, "mock-httpserver")
		assert.Contains(output, "mock-pipeline")
	}

	// egctl get all --namespace mockNamespace
	{
		cmd := egctlCmd("get", "all", "--namespace", "mockNamespace")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.NotContains(output, "httpserver-test")
		assert.NotContains(output, "pipeline-test")
		assert.Contains(output, "mock-httpserver")
		assert.Contains(output, "mock-pipeline")
	}

	// egctl get all --namespace default
	{
		cmd := egctlCmd("get", "all", "--namespace", "default")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "httpserver-test")
		assert.Contains(output, "pipeline-test")
		assert.NotContains(output, "mock-httpserver")
		assert.NotContains(output, "mock-pipeline")
	}

	// egctl get hs --namespace mockNamespace
	{
		cmd := egctlCmd("get", "hs", "--namespace", "mockNamespace")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.NotContains(output, "httpserver-test")
		assert.NotContains(output, "pipeline-test")
		assert.Contains(output, "mock-httpserver")
		assert.NotContains(output, "mock-pipeline")
	}

	// egctl get hs --all-namespaces
	{
		cmd := egctlCmd("get", "hs", "--all-namespaces")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "httpserver-test")
		assert.NotContains(output, "pipeline-test")
		assert.Contains(output, "mock-httpserver")
		assert.NotContains(output, "mock-pipeline")
	}

	// egctl get hs mock-httpserver --namespace mockNamespace -o yaml
	{
		cmd := egctlCmd("get", "hs", "mock-httpserver", "--namespace", "mockNamespace", "-o", "yaml")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.NotContains(output, "httpserver-test")
		assert.NotContains(output, "pipeline-test")
		assert.Contains(output, "mock-httpserver")
		assert.Contains(output, "port: 10222")
	}

	// easegress update status every 5 seconds
	time.Sleep(5 * time.Second)

	// egctl describe httpserver --all-namespaces
	{
		cmd := egctlCmd("describe", "httpserver", "--all-namespaces")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "Name: httpserver-test")
		assert.Contains(output, "Name: mock-httpserver")
		assert.NotContains(output, "Name: pipeline-test")
		assert.NotContains(output, "Name: mock-pipeline")
		assert.Contains(output, "In Namespace default")
		assert.Contains(output, "In Namespace mockNamespace")
		// check if status is updated
		assert.Equal(2, strings.Count(output, "node: primary-single"))
		assert.Equal(2, strings.Count(output, "m1ErrPercent: 0"))
	}

	// egctl describe httpserver --namespace mockNamespace
	{
		cmd := egctlCmd("describe", "httpserver", "--namespace", "mockNamespace")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.Contains(output, "Name: mock-httpserver")
		assert.NotContains(output, "Name: httpserver-test")
		assert.NotContains(output, "Name: pipeline-test")
		assert.NotContains(output, "Name: mock-pipeline")
		// check if status is updated
		assert.Equal(1, strings.Count(output, "node: primary-single"))
		assert.Equal(1, strings.Count(output, "m1ErrPercent: 0"))
	}

	// egctl describe pipeline --namespace default
	{
		cmd := egctlCmd("describe", "pipeline", "--namespace", "default")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.NotContains(output, "Name: httpserver-test")
		assert.Contains(output, "Name: pipeline-test")
		assert.NotContains(output, "Name: mock-httpserver")
		assert.NotContains(output, "Name: mock-pipeline")
		// check if status is updated
		assert.Equal(1, strings.Count(output, "node: primary-single"))
		assert.Equal(1, strings.Count(output, "p999: 0"))
	}

	// egctl describe hs mock-httpserver --namespace mockNamespace -o yaml
	{
		cmd := egctlCmd("describe", "hs", "mock-httpserver", "--namespace", "mockNamespace", "-o", "yaml")
		output, stderr, err := runCmd(cmd)
		assert.Nil(err)
		assert.Empty(stderr)
		assert.NotContains(output, "httpserver-test")
		assert.NotContains(output, "pipeline-test")
		assert.Contains(output, "name: mock-httpserver")
		assert.Contains(output, "port: 10222")
		assert.Contains(output, "node: primary-single")
	}
}

func TestPathEscapeInProxyFilter(t *testing.T) {
	assert := assert.New(t)
	var err error

	yamlStr := `
name: httpserver-escape
kind: HTTPServer
port: 10088
rules:
  - paths:
    - pathPrefix: /
      backend: pipeline-escape
`
	err = createResource(yamlStr)
	assert.NoError(err)
	defer func() {
		err := deleteResource("httpserver", "httpserver-escape")
		assert.NoError(err)
	}()

	yamlStr = `
name: pipeline-escape
kind: Pipeline
flow:
- filter: proxy
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9999
`
	err = createResource(yamlStr)
	assert.NoError(err)
	defer func() {
		err := deleteResource("pipeline", "pipeline-escape")
		assert.NoError(err)
	}()

	ch := make(chan *http.Request, 100)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Must-Start") == "true" {
			return
		}
		fmt.Println("receive url from backend", r.URL.String())
		ch <- r.Clone(context.Background())
	})
	server := mustStartServer(9999, mux, func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:9999", nil)
		req.Header.Add("Must-Start", "true")
		require.Nil(t, err)
		return req
	})
	defer server.Shutdown(context.Background())

	httpServerStarted := checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:10088", nil)
		req.Header.Add("Must-Start", "true")
		require.Nil(t, err)
		return req
	})
	assert.True(httpServerStarted)

	testCases := []struct {
		path  string
		query map[string]string
	}{
		{"/", map[string]string{}},
		{"/中文", map[string]string{"foo": "bar"}},
		{"/with-query?foo=bar", map[string]string{"foo": "中文"}},
		{"/with-fragment#123", map[string]string{"??==##": "!!##?"}},
	}
	for _, tc := range testCases {
		u := url.URL{}
		u.Scheme = "http"
		u.Host = "127.0.0.1:10088"
		u.Path = tc.path
		query := url.Values{}
		for k, v := range tc.query {
			query.Add(k, v)
		}
		u.RawQuery = query.Encode()
		fmt.Println("request url", u.String())

		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		assert.Nil(err)
		resp, err := http.DefaultClient.Do(req)
		assert.Nil(err)
		resp.Body.Close()

		recvReq := <-ch
		fmt.Println("received url from channel", recvReq.URL.String())
		fmt.Println(recvReq.URL)
		assert.Equal(tc.path, recvReq.URL.Path)
		queryMap := map[string]string{}
		for k, v := range recvReq.URL.Query() {
			queryMap[k] = v[0]
		}
		assert.Equal(tc.query, queryMap)
	}
}

func TestGlobalFilterFallthrough(t *testing.T) {
	assert := assert.New(t)

	gfYamlTmpl := `
name: global-filter
kind: GlobalFilter
beforePipeline:
  filters:
  - name: validator
    kind: Validator
    headers:
      Before-Pipeline:
        values: ["valid"]
afterPipeline:
  filters:
  - name: adaptor
    kind: ResponseAdaptor
    header:
      add:
        After-Pipeline: valid
%s
`
	yaml := fmt.Sprintf(gfYamlTmpl, "")
	err := createResource(yaml)
	assert.Nil(err)
	defer deleteResource("globalfilter", "global-filter")

	yaml = `
name: httpserver-gf
kind: HTTPServer
port: 10099
globalFilter: global-filter
rules:
- paths:
  - pathPrefix: /health
    backend: pipeline-ok
  - pathPrefix: /
    backend: pipeline-gf
`
	err = createResource(yaml)
	assert.Nil(err)
	defer deleteResource("httpserver", "httpserver-gf")

	makeReq := func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:10099", nil)
		assert.Nil(err)
		return req
	}

	yaml = `
name: pipeline-ok
kind: Pipeline
filters: 
- name: responsebuilder
  kind: ResponseBuilder
  protocol: http
  template: |
    statusCode: 200
`
	err = createResource(yaml)
	assert.Nil(err)
	defer deleteResource("pipeline", "pipeline-ok")

	// invalid url
	yaml = `
name: pipeline-gf
kind: Pipeline
filters: 
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:11111
`
	err = createResource(yaml)
	assert.Nil(err)
	defer deleteResource("pipeline", "pipeline-gf")

	httpServerStarted := checkServerStart(func() *http.Request {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:10099/health", nil)
		req.Header.Add("Before-Pipeline", "valid")
		require.Nil(t, err)
		return req
	})
	assert.True(httpServerStarted)
	time.Sleep(3 * time.Second)

	// not pass before, not exec pipeline and after
	// bad request from before
	// not header of after
	req := makeReq()
	resp, err := http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusBadRequest, resp.StatusCode, resp)
	assert.Empty(resp.Header.Get("After-Pipeline"), resp)

	// pass before, exec pipeline, meet error, not exec after
	// status code of 503, error in filter
	req = makeReq()
	req.Header.Add("Before-Pipeline", "valid")
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode, resp)
	assert.Empty(resp.Header.Get("After-Pipeline"), resp)

	// pass before, exec pipeline, exec after
	req = makeReq()
	req.URL.Path = "/health"
	req.Header.Add("Before-Pipeline", "valid")
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode, resp)
	assert.Equal("valid", resp.Header.Get("After-Pipeline"), resp)

	// update gfYaml to fallthrough before pipeline
	yaml = fmt.Sprintf(gfYamlTmpl, `
fallthrough:
  beforePipeline: true
`)
	err = applyResource(yaml)
	assert.Nil(err)
	time.Sleep(3 * time.Second)

	// pass before, exec pipeline, meet error, exec after
	// not add header to before
	req = makeReq()
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode, resp)
	assert.Empty(resp.Header.Get("After-Pipeline"), resp)

	// pass before, exec pipeline, exec after
	req = makeReq()
	req.URL.Path = "/health"
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusOK, resp.StatusCode, resp)
	assert.Equal("valid", resp.Header.Get("After-Pipeline"), resp)

	// update gfYaml to fallthrough before pipeline
	yaml = fmt.Sprintf(gfYamlTmpl, `
fallthrough:
  beforePipeline: true
  pipeline: true
`)
	err = applyResource(yaml)
	assert.Nil(err)
	time.Sleep(3 * time.Second)

	// fallthrough before, fallthrough pipeline, exec after
	req = makeReq()
	resp, err = http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Nil(err)
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode, resp)
	assert.Equal("valid", resp.Header.Get("After-Pipeline"), resp)
}
