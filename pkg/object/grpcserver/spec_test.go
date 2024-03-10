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
	"testing"

	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestPort(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: grpc-server-test
kind: GRPCServer
port: 10080
cacheSize: 200
`

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.NoError(err)
	assert.NotNil(superSpec)

	superSpecYaml = `
name: grpc-server-test
kind: GRPCServer
port: 1024
cacheSize: 200
`

	_, err = supervisor.NewSpec(superSpecYaml)
	assert.Error(err)
}

func TestKeepaliveFormat(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: grpc-server-test
kind: GRPCServer
port: 10080
cacheSize: 200
maxConnectionIdle: 60s
`

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.NoError(err)
	assert.NotNil(superSpec)

	superSpecYaml = `
name: grpc-server-test
kind: GRPCServer
port: 10080
cacheSize: 200
maxConnectionIdle: 10
`

	_, err = supervisor.NewSpec(superSpecYaml)
	assert.Error(err)
}

func TestHeaderValidate(t *testing.T) {
	h := &Header{}
	assert.Error(t, h.Validate())
	h.Values = []string{"a"}
	assert.NoError(t, h.Validate())
}
