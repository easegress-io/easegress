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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	assert := assert.New(t)

	// fail to create Pipeline because of invalid yaml
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
	ok, msg := createObject(t, yamlStr)
	assert.False(ok, msg)

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
	ok, msg = createObject(t, yamlStr)
	assert.True(ok, msg)

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
	ok, msg = createObject(t, yamlStr)
	assert.True(ok, msg)

	// list Pipeline and find them by using name
	ok, msg = listObject(t)
	assert.True(ok)
	assert.True(strings.Contains(msg, "name: pipeline-success2"))
	assert.True(strings.Contains(msg, "name: pipeline-success1"))

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
	ok, msg = updateObject(t, "pipeline-success2", yamlStr)
	assert.True(ok, msg)

	ok, msg = listObject(t)
	assert.True(ok)
	assert.True(strings.Contains(msg, "http://update-pipeline-success2:8888"))

	// delete all Pipelines
	ok, msg = deleteObject(t, "pipeline-success1")
	assert.True(ok, msg)
	ok, msg = deleteObject(t, "pipeline-success2")
	assert.True(ok, msg)

	ok, msg = listObject(t)
	assert.True(ok)
	assert.False(strings.Contains(msg, "name: pipeline-success1"))
	assert.False(strings.Contains(msg, "name: pipeline-success2"))
}

func TestHTTPServer(t *testing.T) {
	assert := assert.New(t)

	// fail to create HTTPServer because of invalid yaml
	yamlStr := `
name: httpserver-fail    
kind: HTTPServer
`
	ok, msg := createObject(t, yamlStr)
	assert.False(ok, msg)

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
	ok, msg = createObject(t, yamlStr)
	assert.True(ok, msg)

	// list HTTPServer and find it by name
	ok, msg = listObject(t)
	assert.True(ok)
	assert.True(strings.Contains(msg, "name: httpserver-success"))

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
	ok, msg = updateObject(t, "httpserver-success", yamlStr)
	assert.True(ok, msg)

	ok, msg = listObject(t)
	assert.True(ok)
	assert.True(strings.Contains(msg, "backend: update-httpserver-success"))

	// delete all HTTPServer
	ok, msg = deleteObject(t, "httpserver-success")
	assert.True(ok, msg)

	ok, msg = listObject(t)
	assert.True(ok)
	assert.False(strings.Contains(msg, "name: httpserver-success"))
}
