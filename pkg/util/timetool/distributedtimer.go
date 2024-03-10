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

// Package timetool provides time utilities.
package timetool

import (
	"time"
)

type (
	// NextDurationFunc is the type for registering callback function
	// to determinate the timer to tick after the returned duration.
	NextDurationFunc func() time.Duration

	// DistributedTimer keeps timer's tick time consistent.
	// Please notice nextDurationFunc assumes different instances
	// running under the same system clock.
	DistributedTimer struct {
		nextDurationFunc NextDurationFunc

		C chan time.Time

		done chan struct{}
	}
)

// NewDistributedTimer creates a DistributedTimer
func NewDistributedTimer(nextDurationFunc NextDurationFunc) *DistributedTimer {
	dt := &DistributedTimer{
		nextDurationFunc: nextDurationFunc,
		C:                make(chan time.Time),
		done:             make(chan struct{}),
	}

	go dt.run()

	return dt
}

func (dt *DistributedTimer) run() {
	for {
		select {
		case <-dt.done:
			return
		case now := <-time.After(dt.nextDurationFunc()):
			// NOTE: Use a select-statement to avoid block and always send last time.
			select {
			case dt.C <- now:
			default:
			}
		}
	}
}

// Close closes DistributedTimer.
func (dt *DistributedTimer) Close() {
	close(dt.done)
}
