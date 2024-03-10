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

package proxies

import (
	"sync"
	"sync/atomic"
)

type MockHealthChecker struct {
	Expect int32
	Result bool
	WG     *sync.WaitGroup

	spec    HealthCheckSpec
	counter int32
}

func (c *MockHealthChecker) BaseSpec() HealthCheckSpec {
	return c.spec
}

func (c *MockHealthChecker) Check(svr *Server) bool {
	if c.WG != nil && atomic.AddInt32(&c.counter, 1) <= c.Expect {
		c.WG.Done()
	}
	return c.Result
}

func (c *MockHealthChecker) Close() {
}
