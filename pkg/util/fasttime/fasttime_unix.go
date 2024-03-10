//go:build !windows
// +build !windows

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

package fasttime

import (
	"syscall"
	"time"
)

// Now is the current time
func Now() time.Time {
	var tv syscall.Timeval
	syscall.Gettimeofday(&tv)
	return time.Unix(0, syscall.TimevalToNsec(tv))
}

// NowUnixNano is the current Unix Nano time
func NowUnixNano() int64 {
	var tv syscall.Timeval
	syscall.Gettimeofday(&tv)
	return syscall.TimevalToNsec(tv)
}
