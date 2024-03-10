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
	"os"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestHTTPServer(t *testing.T) {
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

	svr := &HTTPServer{}
	svr.Init(superSpec, &contexttest.MockedMuxMapper{})

	yamlConfig = `
kind: HTTPServer
name: test
port: 38082
keepAlive: true
https: false
`
	superSpec, err = super.NewSpec(yamlConfig)
	assert.NoError(err)
	svr2 := &HTTPServer{}
	svr2.Inherit(superSpec, svr, &contexttest.MockedMuxMapper{})

	time.Sleep(200 * time.Millisecond)
	assert.NotNil(svr2.Status())
	svr2.Close()
	time.Sleep(200 * time.Millisecond)
}
