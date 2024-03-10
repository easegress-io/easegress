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
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func resetEnv() func() {
	envs := os.Environ()
	return func() {
		os.Clearenv()
		for _, env := range envs {
			kv := strings.SplitN(env, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
	}
}

func TestSetFlagsFromEnv(t *testing.T) {
	assert := assert.New(t)

	reset := resetEnv()
	defer reset()
	os.Setenv("EASEGRESS_TEST_STRING", "test")
	os.Setenv("EASEGRESS_TEST_BOOL", "true")
	os.Setenv("EASEGRESS_TEST_ARRAY", "a,b,c")
	os.Setenv("EASEGRESS_TEST_INT", "1")

	var stringFlag string
	var boolFlag bool
	var arrayFlag []string
	var intFlag int
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringVar(&stringFlag, "test-string", "", "test string flag")
	flags.BoolVar(&boolFlag, "test-bool", false, "test bool flag")
	flags.StringSliceVar(&arrayFlag, "test-array", []string{}, "test array flag")
	flags.IntVar(&intFlag, "test-int", 0, "test int flag")

	err := SetFlagsFromEnv("EASEGRESS", flags)
	assert.NoError(err)
	assert.Equal("test", stringFlag)
	assert.Equal(true, boolFlag)
	assert.Equal([]string{"a", "b", "c"}, arrayFlag)
	assert.Equal(1, intFlag)
}

func TestSetFlagsFromEnvFail(t *testing.T) {
	assert := assert.New(t)

	reset := resetEnv()
	defer reset()
	os.Setenv("EASEGRESS_TEST_BOOL", "123")

	var boolFlag bool
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.BoolVar(&boolFlag, "test-bool", false, "test bool flag")

	err := SetFlagsFromEnv("EASEGRESS", flags)
	assert.Error(err)

	os.Setenv("EASEGRESS_VERSION", "not-a-bool")
	// options from env
	opt := New()
	err = opt.Parse()
	assert.Error(err)
}

func TestOptionFromEnv(t *testing.T) {
	assert := assert.New(t)

	reset := resetEnv()
	defer reset()
	os.Setenv("EASEGRESS_NAME", "test-option-from-env")
	os.Setenv("EASEGRESS_HOME_DIR", "./test-home-dir")
	os.Setenv("EASEGRESS_LOG_DIR", "./test-log-dir")
	os.Setenv("EASEGRESS_NAME", "test-option-from-env")
	os.Setenv("EASEGRESS_LISTEN_CLIENT_URLS", "http://localhost:2379,http://localhost:10080")

	{
		// options from env
		opt := New()
		err := opt.Parse()
		assert.NoError(err)
		assert.Equal("test-option-from-env", opt.Name)
		assert.Equal("./test-home-dir", opt.HomeDir)
		assert.Equal("./test-log-dir", opt.LogDir)
		assert.Equal([]string{"http://localhost:2379", "http://localhost:10080"}, opt.Cluster.ListenClientURLs)
	}

	{
		args := os.Args
		defer func() {
			os.Args = args
		}()
		os.Args = []string{"easegress-server", "--name=test-option-from-args", "--home-dir=./test-home-dir-from-args", "--log-dir=./test-log-dir-from-args"}
		// options from env and args
		opt := New()
		err := opt.Parse()
		assert.NoError(err)
		assert.Equal("test-option-from-args", opt.Name)
		assert.Equal("./test-home-dir-from-args", opt.HomeDir)
		assert.Equal("./test-log-dir-from-args", opt.LogDir)
		assert.Equal([]string{"http://localhost:2379", "http://localhost:10080"}, opt.Cluster.ListenClientURLs)
	}
}
