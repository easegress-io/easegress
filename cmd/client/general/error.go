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

package general

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

// ExitWithError exits with self-defined message not the one of cobra(such as usage).
func ExitWithError(err error) {
	if err != nil {
		color.New(color.FgRed).Fprint(os.Stderr, "Error: ")
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// ExitWithErrorf wraps ExitWithError with format.
func ExitWithErrorf(format string, a ...interface{}) {
	ExitWithError(fmt.Errorf(format, a...))
}
