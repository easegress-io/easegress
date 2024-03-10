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

// Package general provides the general utilities for the client.
package general

import (
	"fmt"
	"strings"
)

// SuccessMsg returns the success message.
func SuccessMsg(action CmdType, values ...string) string {
	return fmt.Sprintf("%s %s successfully", action, strings.Join(values, " "))
}

// ErrorMsg returns the error message.
func ErrorMsg(action CmdType, err error, values ...string) error {
	return fmt.Errorf("%s %s failed, %v", action, strings.Join(values, " "), err)
}

// Warnf prints the warning message.
func Warnf(format string, args ...interface{}) {
	fmt.Printf("WARNING: "+format+"\n", args...)
}

// Infof prints the info message.
func Infof(format string, args ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", args...)
}
