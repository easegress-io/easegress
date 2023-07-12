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
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestLogFileWrite(t *testing.T) {
	dir := makeTempDir("TestLogFileWrite", t)
	path := logFilePath(dir)
	defer os.RemoveAll(dir)

	file, err := NewLogFile(&Spec{
		FileName: path,
	}, 1024)
	at := assert.New(t)
	at.NoError(err)
	b := []byte("TestLogFileWrite")
	written, err := file.Write(b)
	at.NoError(err)
	at.Equal(written, len(b))
	// wait timer flush
	time.Sleep(2 * cacheTimeout)
	existsWithContent(path, b, t)
}

func TestLogFileClose(t *testing.T) {
	dir := makeTempDir("TestLogFileClose", t)
	path := logFilePath(dir)
	defer os.RemoveAll(dir)

	file, _ := NewLogFile(&Spec{
		FileName: path,
	}, 0)
	at := assert.New(t)
	b := []byte("TestLogFileClose")
	file.Write(b)

	file.Close()
	existsWithContent(path, b, t)
	_, ok := <-file.eventChan

	at.False(ok)
}
