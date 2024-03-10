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

package nginx

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCmd(t *testing.T) {
	cmd := Cmd()
	assert.NotNil(t, cmd)
	cmd.ParseFlags([]string{""})
	assert.NotNil(t, cmd.Args(cmd, []string{}))

	cmd.ParseFlags([]string{"-f", "test.conf"})
	assert.NotNil(t, cmd.Args(cmd, []string{}))

	cmd.ParseFlags([]string{"-o", "test.yaml"})
	assert.Nil(t, cmd.Args(cmd, []string{}))

	cmd.ParseFlags([]string{"--resource-prefix", "test"})
	assert.Nil(t, cmd.Args(cmd, []string{}))

	tempDir := newTempTestDir(t)
	defer tempDir.Clean()

	nginxConf := `
	events {}
	http {
		server {
			listen 127.0.0.1:8080;

			location = /user {
				proxy_pass http://localhost:9999;
			}
		}
	}
	`
	nginxFile := tempDir.Create("nginx.conf", []byte(nginxConf))
	outputFile := tempDir.Create("test.yaml", []byte(""))
	cmd.ParseFlags([]string{"-f", nginxFile, "-o", outputFile, "--prefix", "test"})
	cmd.Run(cmd, []string{})
	file, err := os.Open(outputFile)
	assert.Nil(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	assert.Nil(t, err)
	assert.Contains(t, string(data), "test-8080")
	assert.Contains(t, string(data), "test-user")
}

func TestOption(t *testing.T) {
	option := &Options{
		NginxConf:      "test.conf",
		Output:         "test.yaml",
		ResourcePrefix: "test",
	}
	option.init()
	path := option.GetPipelineName("/user")
	assert.Equal(t, "test-user", path)
	path = option.GetPipelineName("/apis/v1")
	assert.Equal(t, "test-apisv1", path)

	path = option.GetPipelineName("/apis/v1/")
	assert.Contains(t, path, "test-apisv1")
	assert.NotEqual(t, "test-apisv1", path)
}
