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

package grpcprxoy

import (
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func newTestProxy(yamlSpec string, assert *assert.Assertions) *Proxy {
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlSpec), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assert.NoError(err)

	proxy := kind.CreateInstance(spec).(*Proxy)
	proxy.Init()

	assert.Equal(kind, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func TestInvalidSpec(t *testing.T) {
	assertions := assert.New(t)
	s := `
kind: GRPCProxy
useConnectionPool: true
maxConnsPerHost: 0
pools:
  - loadBalance:
      policy: forward
    serviceName: "easegress"
name: grpcforwardproxy
`
	rawSpec := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(s), &rawSpec)
	assertions.NoError(err)

	_, err = filters.NewSpec(nil, "", rawSpec)
	assertions.Error(err)

	s = `
kind: GRPCProxy
useConnectionPool: true
maxConnsPerHost: 1
borrowTimeout: 3s
connectTimeout: 3s
pools:
  - loadBalance:
      policy: forward
    serviceName: "easegress"
name: grpcforwardproxy
`
	rawSpec = make(map[string]interface{})
	err = yaml.Unmarshal([]byte(s), &rawSpec)
	assertions.NoError(err)

	_, err = filters.NewSpec(nil, "", rawSpec)
	assertions.Error(err)
}

func TestReload(t *testing.T) {
	assertions := assert.New(t)
	s := `
kind: GRPCProxy
useConnectionPool: true
maxConnsPerHost: 1
borrowTimeout: 2000ms
connectTimeout: 1000ms
pools:
  - loadBalance:
      policy: forward
    serviceName: "easegress"
name: grpcforwardproxy
`
	p := newTestProxy(s, assertions)
	assertions.Zero(getSyncMapSize(&p.conns))
	assertions.NotNil(p.pool)

	s = `
kind: GRPCProxy
useConnectionPool: true
maxConnsPerHost: 1
borrowTimeout: 2000ms
connectTimeout: 1000ms
pools:
  - loadBalance:
      policy: forward
    serviceName: "easegress"
name: grpcforwardproxy
`

	p = newTestProxy(s, assertions)
	assertions.Zero(getSyncMapSize(&p.conns))
	assertions.NotNil(p.pool)

	p.Close()
	assertions.Zero(getSyncMapSize(&p.conns))
	assertions.Nil(p.pool)
}
