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

package logger

import (
	"github.com/megaease/easegress/pkg/option"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"
)

func TestLogOptions(t *testing.T) {
	at := assert.New(t)
	options := option.New()
	dir := path.Join(os.TempDir(), "TestLogOptions")
	options.HomeDir = dir
	options.LogDir = "logs"
	options.DisableAccessLog = true
	defer os.RemoveAll(dir)

	at.NoError(options.Parse())
	abs, err := filepath.Abs(path.Join(dir, "logs"))
	at.NoError(err)
	at.Equal(abs, options.AbsLogDir)

	initHTTPFilter(options)

	HTTPAccess("TestLogOptions http access log")
	// wait two cycles of logger.cacheTimeout
	time.Sleep(4*time.Second + 100*time.Millisecond)

	_, err = os.Stat(dir)
	at.True(os.IsNotExist(err))
}
