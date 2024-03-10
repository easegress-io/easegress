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

package timetool

import (
	"testing"
	"time"
)

func TestDistributedTime(t *testing.T) {
	dt := NewDistributedTimer(func() time.Duration {
		return 20 * time.Millisecond
	})

	<-dt.C
	time.Sleep(30 * time.Millisecond)
	<-dt.C
	dt.Close()
	time.Sleep(10 * time.Millisecond)
}
