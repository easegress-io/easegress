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

package grpcserver

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

func TestNewGrpcServer(t *testing.T) {
	assertions := assert.New(t)
	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assertions.NoError(err)

	svr := &GRPCServer{}
	svr.Init(superSpec, &contexttest.MockedMuxMapper{})

	yamlSpec = `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8851
name: server-grpc
`
	superSpec, err = supervisor.NewSpec(yamlSpec)
	assertions.NoError(err)

	svr2 := &GRPCServer{}
	svr2.Inherit(superSpec, svr, &contexttest.MockedMuxMapper{})

	time.Sleep(500 * time.Millisecond)
	s := svr2.Status().ObjectStatus.(*Status)
	assertions.True(s.Health)
	assertions.Empty(s.Error)
	svr2.Close()
	assertions.Equal(stateClosed, svr2.Status().ObjectStatus.(*Status).State)
}
