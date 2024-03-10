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
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func prepareServers(count int) []*Server {
	svrs := make([]*Server, 0, count)
	for i := 0; i < count; i++ {
		svrs = append(svrs, &Server{Weight: i + 1, URL: fmt.Sprintf("192.168.1.%d", i+1)})
	}
	return svrs
}

func TestGeneralLoadBalancer(t *testing.T) {
	serverCount := 10
	servers := prepareServers(serverCount)
	spec := &LoadBalanceSpec{
		Policy: LoadBalancePolicyRoundRobin,
		StickySession: &StickySessionSpec{
			Mode:          StickySessionModeCookieConsistentHash,
			AppCookieName: "app_cookie",
		},
		HealthCheck: &HealthCheckSpec{
			Interval: "5ms",
		},
	}

	lb := NewGeneralLoadBalancer(spec, servers)
	wg := &sync.WaitGroup{}
	wg.Add(serverCount)
	hc := &MockHealthChecker{Expect: int32(serverCount), WG: wg, Result: false}
	lb.Init(NewHTTPSessionSticker, hc, nil)
	wg.Wait()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, len(lb.healthyServers.Load().Servers), 0)

	lb.Close()

	servers = prepareServers(10)
	lb = NewGeneralLoadBalancer(spec, servers)

	wg.Add(serverCount)
	hc = &MockHealthChecker{Expect: int32(serverCount), WG: wg, Result: true}
	lb.Init(NewHTTPSessionSticker, hc, nil)
	wg.Wait()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, len(lb.healthyServers.Load().Servers), 10)
	lb.Close()
}

func TestRandomLoadBalancePolicy(t *testing.T) {
	counter := [10]int{}
	servers := prepareServers(10)

	lb := NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyRandom}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 100000; i++ {
		svr := lb.ChooseServer(nil)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		if v := counter[i]; v < 8000 || v > 12000 {
			t.Errorf("possibility is not even with value %v", v)
		}
	}
}

func TestRoundRobinLoadBalancePolicy(t *testing.T) {
	servers := prepareServers(10)

	lb := NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyRoundRobin}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 10; i++ {
		svr := lb.ChooseServer(nil)
		assert.Equal(t, svr.Weight, i+1)
	}

	lb = NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: "UnknowPolicy"}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 10; i++ {
		svr := lb.ChooseServer(nil)
		assert.Equal(t, svr.Weight, i+1)
	}
}

func TestWeightedRandomLoadBalancePolicy(t *testing.T) {
	counter := [10]int{}
	servers := prepareServers(10)

	lb := NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyWeightedRandom}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 50000; i++ {
		svr := lb.ChooseServer(nil)
		counter[svr.Weight-1]++
	}

	v := 0
	for i := 0; i < 10; i++ {
		if v >= counter[i] {
			t.Errorf("possibility is not weighted even %v", counter)
		}
		v = counter[i]
	}
}

func TestIPHashLoadBalancePolicy(t *testing.T) {
	counter := [10]int{}
	servers := prepareServers(10)

	lb := NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyIPHash}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.Header.Add("X-Real-Ip", fmt.Sprintf("192.168.1.%d", i+1))
		r, _ := httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		assert.GreaterOrEqual(t, counter[i], 1)
	}
}

func TestHeaderHashLoadBalancePolicy(t *testing.T) {
	counter := [10]int{}
	servers := prepareServers(10)

	lb := NewGeneralLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyHeaderHash, HeaderHashKey: "X-Header"}, servers)
	lb.Init(nil, nil, nil)

	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.Header.Add("X-Header", fmt.Sprintf("abcd-%d", i))
		r, _ := httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		assert.GreaterOrEqual(t, counter[i], 1)
	}
}
