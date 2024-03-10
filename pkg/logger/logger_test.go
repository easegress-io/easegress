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

package logger

import (
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/option"
)

func TestMustPlainLogger(t *testing.T) {
	opt := &option.Options{AbsLogDir: "."}
	defer func() {
		if rv := recover(); rv != nil {
			t.Errorf("mustPlainLogger() panic: %v", rv)
		}
	}()

	MustNewPlainLogger(opt, "test.log", 1)
	_, err := os.Stat("test.log")
	if err == nil {
		return
	}

	if os.IsNotExist(err) {
		t.Errorf("mustPlainLogger() failed: %v", err)
	}
}

func TestMustPlainLoggerPanic(t *testing.T) {
	opt := &option.Options{AbsLogDir: "."}
	defer func() {
		if rv := recover(); rv != nil {
			t.Logf("mustPlainLogger() panic: %v", rv)
		}
	}()

	MustNewPlainLogger(opt, "test.log", 0)
	_, err := os.Stat("test.log")
	if err == nil {
		return
	}

	if os.IsNotExist(err) {
		t.Errorf("mustPlainLogger() failed: %v", err)
	}
}

func TestMustPlainLoggerWrite(t *testing.T) {
	opt := &option.Options{AbsLogDir: "."}
	defer func() {
		if rv := recover(); rv != nil {
			t.Errorf("mustPlainLogger() panic: %v", rv)
		}
	}()

	l := MustNewPlainLogger(opt, "test.log", 0)
	l.Errorf("test error")
	l.Info("test info")
	l.Debug("test debug")
	l.Sync()
	t.Logf("mustPlainLogger() success")
}
