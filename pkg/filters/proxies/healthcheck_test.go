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

package proxies

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockHealthChecker struct {
	expect  int32
	counter int32
	result  bool
	wg      *sync.WaitGroup
}

func (c *MockHealthChecker) Check(svr *Server) bool {
	if c.wg != nil && atomic.AddInt32(&c.counter, 1) <= c.expect {
		c.wg.Done()
	}
	return c.result
}

func (c *MockHealthChecker) Close() {
}

func TestHTTPHealthChecker(t *testing.T) {
	spec := &HealthCheckSpec{}
	c := NewHTTPHealthChecker(spec)
	assert.NotNil(t, c)

	spec = &HealthCheckSpec{Timeout: "100ms"}
	c = NewHTTPHealthChecker(spec)
	c.Check(&Server{URL: "https://www.megaease.com"})

	c.Close()
}
