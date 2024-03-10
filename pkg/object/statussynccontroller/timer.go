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

package statussynccontroller

import (
	"time"
)

const (
	// SyncStatusPaceInUnixSeconds must be 5s because the rates of http stat.
	// https://github.com/rcrowley/go-metrics/blob/3113b8401b8a98917cde58f8bbd42a1b1c03b1fd/ewma.go#L98-L99
	SyncStatusPaceInUnixSeconds = 5
)

func nextSyncStatusDuration() time.Duration {
	return nextDuration(time.Now(), SyncStatusPaceInUnixSeconds)
}

// nextDuration returns the next duration after t,
// plus satisfying the pace in UNIX seconds(minimum unit for now).
func nextDuration(t time.Time, paceInUnixSeconds int) time.Duration {
	paceInNano := int64(paceInUnixSeconds) * int64(time.Second/time.Nanosecond)
	rounds := t.UnixNano() / paceInNano

	return time.Duration(paceInNano*(rounds+1) - t.UnixNano())
}
