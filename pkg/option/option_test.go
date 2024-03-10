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

package option

import (
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"

	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestLogOptions(t *testing.T) {
	at := assert.New(t)
	options := New()
	dir := path.Join(os.TempDir(), "TestLogOptions")
	options.HomeDir = dir
	options.LogDir = "logs"
	defer os.RemoveAll(dir)

	at.NoError(options.Parse())
	abs, err := filepath.Abs(path.Join(dir, "logs"))
	at.NoError(err)
	at.Equal(abs, options.AbsLogDir)
	at.False(options.DisableAccessLog)
}

func TestOption(t *testing.T) {
	assert := assert.New(t)
	options := New()
	assert.NoError(options.Parse())

	// test YAML, and FlagUsages method
	{
		yamlStr := options.YAML()
		assert.NotEmpty(yamlStr)

		data, err := codectool.MarshalYAML(options)
		assert.NoError(err)
		assert.NotEmpty(data)
		assert.Equal(yamlStr, string(data))

		flagUsages := options.FlagUsages()
		assert.NotEmpty(flagUsages)
		assert.Equal(options.flags.FlagUsages(), flagUsages)
	}

	// check rename legacy cluster roles
	{
		options.flags.Parse([]string{"--cluster-role=writer"})
		assert.Equal("writer", options.ClusterRole)
		options.renameLegacyClusterRoles()
		assert.Equal("primary", options.ClusterRole)

		options.flags.Parse([]string{"--cluster-role=reader"})
		assert.Equal("reader", options.ClusterRole)
		options.renameLegacyClusterRoles()
		assert.Equal("secondary", options.ClusterRole)
	}

	// test show version, always success parse
	{
		options.ShowVersion = true
		assert.NoError(options.Parse())
		options.ShowVersion = false
	}

	// test config file not exist, return error
	{
		options.ConfigFile = "not-exist"
		assert.Error(options.Parse())
	}

	// check validate
	{
		assert.Nil(options.validate())

		// invalid cluster name
		func() {
			name := options.ClusterName
			defer func() {
				options.ClusterName = name
			}()

			options.ClusterName = ""
			assert.Error(options.validate())
			options.ClusterName = "!!!not-valid-name"
			assert.Error(options.validate())
		}()

		// invalid cluster role, secondary can not force new cluster
		func() {
			role := options.ClusterRole
			force := options.ForceNewCluster
			defer func() {
				options.ClusterRole = role
				options.ForceNewCluster = force
			}()

			options.ClusterRole = "secondary"
			options.ForceNewCluster = true
			assert.Error(options.validate())
		}()

		// invalid cluster role, secondary need primary listen peer urls
		func() {
			role := options.ClusterRole
			urls := options.Cluster.PrimaryListenPeerURLs
			defer func() {
				options.ClusterRole = role
				options.Cluster.PrimaryListenPeerURLs = urls
			}()

			options.ClusterRole = "secondary"
			options.Cluster.PrimaryListenPeerURLs = []string{}
			assert.Error(options.validate())
		}()

		// invalid cluster role
		func() {
			role := options.ClusterRole
			defer func() {
				options.ClusterRole = role
			}()

			options.ClusterRole = "not-exist"
			assert.Error(options.validate())
		}()

		// invalid home dir
		func() {
			dir := options.HomeDir
			defer func() {
				options.HomeDir = dir
			}()

			options.HomeDir = ""
			assert.Error(options.validate())
		}()

		// invalid data dir
		func() {
			dir := options.DataDir
			defer func() {
				options.DataDir = dir
			}()

			options.DataDir = ""
			assert.Error(options.validate())
		}()

		// invalid tls cert file
		func() {
			tlsFlag := options.TLS
			defer func() {
				options.TLS = tlsFlag
			}()

			options.TLS = true
			assert.Error(options.validate())
		}()

		// invalid name
		func() {
			name := options.Name
			apiAddr := options.APIAddr
			defer func() {
				options.Name = name
				options.APIAddr = apiAddr
			}()

			options.Name = ""
			options.APIAddr = ""
			assert.Error(options.validate())
		}()

		assert.Nil(options.validate())
	}

	// other methods
	{
		assert.Equal("eg-default-name=http://localhost:2380", options.InitialClusterToString())
		assert.Equal([]string{"http://localhost:2380"}, options.GetPeerURLs())
		urls, err := options.GetFirstAdvertiseClientURL()
		assert.Nil(err)
		assert.Equal("http://localhost:2379", urls)
	}
}
