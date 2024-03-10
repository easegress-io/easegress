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

package common

import (
	"fmt"
	"os"
)

// Exit the call exit system call with exit code and message
func Exit(code int, msg string) {
	if code != 0 {
		if msg != "" {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
		os.Exit(code)
	}

	if msg != "" {
		fmt.Fprintf(os.Stdout, "%v\n", msg)
	}
	os.Exit(0)
}
