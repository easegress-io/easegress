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

package tcpproxy

import (
	"os"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestDefaultSpec(t *testing.T) {
	assertions := assert.New(t)
	tp := &TCPProxy{}
	spec := tp.DefaultSpec().(*Spec)
	assertions.Equal(uint16(8080), spec.Port)
	assertions.Equal(uint32(10240), spec.MaxConnections)
	assertions.Equal("roundRobin", spec.LoadBalance)
}

func TestCategoryAndKind(t *testing.T) {
	assertions := assert.New(t)
	tp := &TCPProxy{}
	assertions.Equal(string(supervisor.CategoryTrafficGate), string(tp.Category()))
	assertions.Equal(Kind, tp.Kind())
}

func TestSpecValidation(t *testing.T) {
	testCases := []struct {
		name    string
		spec    *Spec
		wantErr bool
	}{
		{
			name: "valid spec with round robin",
			spec: &Spec{
				Port:        8080,
				LoadBalance: LoadBalanceRoundRobin,
				Backends: []*Backend{
					{Address: "127.0.0.1:8081"},
					{Address: "127.0.0.1:8082"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid spec with weighted round robin",
			spec: &Spec{
				Port:        8080,
				LoadBalance: LoadBalanceWeightedRoundRobin,
				Backends: []*Backend{
					{Address: "127.0.0.1:8081", Weight: 3},
					{Address: "127.0.0.1:8082", Weight: 1},
				},
			},
			wantErr: false,
		},
		{
			name: "missing port",
			spec: &Spec{
				Backends: []*Backend{
					{Address: "127.0.0.1:8081"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty backends",
			spec: &Spec{
				Port:     8080,
				Backends: []*Backend{},
			},
			wantErr: true,
		},
		{
			name: "nil backends",
			spec: &Spec{
				Port: 8080,
			},
			wantErr: true,
		},
		{
			name: "empty backend address",
			spec: &Spec{
				Port: 8080,
				Backends: []*Backend{
					{Address: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid load balance strategy",
			spec: &Spec{
				Port:        8080,
				LoadBalance: "unknown",
				Backends: []*Backend{
					{Address: "127.0.0.1:8081"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spec.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewTCPProxy(t *testing.T) {
	assertions := assert.New(t)
	yamlSpec := `
kind: TCPProxy
port: 18888
maxConnections: 1024
connectTimeout: 5s
idleTimeout: 30s
loadBalance: roundRobin
backends:
  - address: 127.0.0.1:18889
  - address: 127.0.0.1:18890
name: test-tcpproxy
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assertions.NoError(err)

	tp := &TCPProxy{}
	tp.Init(superSpec, &contexttest.MockedMuxMapper{})

	time.Sleep(500 * time.Millisecond)
	s := tp.Status().ObjectStatus.(*Status)
	assertions.True(s.Health)
	assertions.Empty(s.Error)
	tp.Close()
	assertions.Equal(stateClosed, tp.Status().ObjectStatus.(*Status).State)
}

func TestTCPProxyReload(t *testing.T) {
	assertions := assert.New(t)
	yamlSpec := `
kind: TCPProxy
port: 18891
maxConnections: 1024
loadBalance: roundRobin
backends:
  - address: 127.0.0.1:18892
name: test-tcpproxy-reload
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assertions.NoError(err)

	tp := &TCPProxy{}
	tp.Init(superSpec, &contexttest.MockedMuxMapper{})
	time.Sleep(500 * time.Millisecond)

	s := tp.Status().ObjectStatus.(*Status)
	assertions.True(s.Health)

	// Reload with updated maxConnections (should not restart).
	yamlSpec2 := `
kind: TCPProxy
port: 18891
maxConnections: 2048
loadBalance: roundRobin
backends:
  - address: 127.0.0.1:18892
name: test-tcpproxy-reload
`
	superSpec2, err := supervisor.NewSpec(yamlSpec2)
	assertions.NoError(err)

	tp2 := &TCPProxy{}
	tp2.Inherit(superSpec2, tp, &contexttest.MockedMuxMapper{})

	time.Sleep(300 * time.Millisecond)
	s = tp2.Status().ObjectStatus.(*Status)
	assertions.True(s.Health)

	tp2.Close()
	assertions.Equal(stateClosed, tp2.Status().ObjectStatus.(*Status).State)
}
