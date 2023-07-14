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
	"github.com/agiledragon/gomonkey/v2"
	"github.com/megaease/easegress/pkg/env"
	"github.com/megaease/easegress/pkg/option"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestInitLogger(t *testing.T) {
	convey.Convey("Test Mock os.ReadFile", t, func() {
		y := `
stdLog:
  fileName: std.log
  maxSize: 1
  maxBackups: 2
  perm: "0600"
`
		patches := gomonkey.ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
			return []byte(y), nil
		})
		defer patches.Reset()
		dir := makeTempDir("TestInitLogger", t)
		defer os.RemoveAll(dir)

		options := option.New()
		options.HomeDir = dir
		options.LogDir = "logs"
		options.LogConfig = "123"
		convey.So(options.Parse(), convey.ShouldBeNil)
		convey.So(env.InitServerDir(options), convey.ShouldBeNil)

		convey.So(func() { Init(options) }, convey.ShouldNotPanic)
		// reset the loggers
		defer InitNop()

		convey.So(defaultLogsConfig.StdLog.MaxSize, convey.ShouldEqual, 1)
		convey.So(defaultLogsConfig.StdLog.Perm, convey.ShouldEqual, "0600")
		convey.So(defaultLogsConfig.EtcdServer, convey.ShouldNotBeNil)

		convey.So(defaultLogger, convey.ShouldNotBeNil)
		convey.So(etcdClientLogger, convey.ShouldNotBeNil)
		convey.So(CustomerEtcdClientLogger(options, "test-unit"), convey.ShouldNotBeNil)

		fileCount(path.Join(dir, "logs"), 0, t)
	})

}

func TestInitMockAndMock(t *testing.T) {
	at := assert.New(t)
	at.NotPanics(InitMock)
	at.NotPanics(InitNop)
}
