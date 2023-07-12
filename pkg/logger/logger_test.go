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
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	y := `
stdLog:
  fileName: std.log
  maxSize: 1
  maxBackups: 2
  perm: "0600"
`
	codectool.MustUnmarshal([]byte(y), defaultLogsConfig)

	at := assert.New(t)
	at.Equal("std.log", defaultLogsConfig.StdLog.FileName)
	at.Equal("0600", defaultLogsConfig.StdLog.Perm)
	at.Equal(1, defaultLogsConfig.StdLog.MaxSize)

	at.NotNil(defaultLogsConfig.EtcdServer)
	at.NotNil(defaultLogsConfig.Access)
	at.NotNil(defaultLogsConfig.Dump)
	at.NotNil(defaultLogsConfig.AdminAPI)
	at.NotNil(defaultLogsConfig.OTel)
	at.NotNil(defaultLogsConfig.EtcdClient)
}
