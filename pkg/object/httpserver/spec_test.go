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

package httpserver

import (
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"

	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func TestValidate(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: http-server-test
kind: HTTPServer
port: 10080
cacheSize: 200
rules:
  - paths:
    - pathPrefix: /api
`

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	assert.NotNil(superSpec.ObjectSpec())

	superSpecYaml = `
name: http-server-test
kind: HTTPServer
port: 10080
cacheSize: 200
rules:
  - paths:
    - rewriteTarget: /api
`

	superSpec, err = supervisor.NewSpec(superSpecYaml)
	assert.True(strings.Contains(err.Error(), "rewriteTarget is specified but path is empty"))
	assert.Nil(superSpec)
}
