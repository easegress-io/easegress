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

package httpserver

import (
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestNewRuntim(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `
kind: HTTPServer
name: test
port: 38081
keepAlive: true
https: false
`
	super := supervisor.NewMock(option.New(), nil, nil,
		nil, false, nil, nil)
	superSpec, err := super.NewSpec(yamlConfig)
	assert.NoError(err)

	r := newRuntime(superSpec, &contexttest.MockedMuxMapper{})
	assert.NotNil(r)

	yamlConfig = `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
cacheSize: 100
tracing:
  serviceName: test
  sampleRate: 0.1
  exporter:
    kind: zipkin
    zipkin:
      collectorURL: http://test.megaease.com/zipkin

rules:
- host: www.megaease.com
  paths:
  - path: /abc
    backend: abc-pipeline
- host: www.megaease.cn
  paths:
  - pathPrefix: /xyz
    backend: xyz-pipeline
`
	superSpec, err = supervisor.NewSpec(yamlConfig)
	assert.NoError(err)

	r.reload(superSpec, &contexttest.MockedMuxMapper{})

	time.Sleep(500 * time.Millisecond)

	assert.NotNil(r.Status())

	yamlConfig = `
kind: HTTPServer
name: test
port: 38082
keepAlive: true
https: false
`
	superSpec, err = supervisor.NewSpec(yamlConfig)
	assert.NoError(err)
	assert.True(r.needRestartServer(superSpec.ObjectSpec().(*Spec)))

	res := r.Status().ToMetrics("mock")
	assert.NotNil(res)

	r.Close()
	time.Sleep(100 * time.Millisecond)

	//
}
