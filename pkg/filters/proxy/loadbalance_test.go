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

package proxy

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func prepareServers(count int) []*Server {
	svrs := make([]*Server, 0, count)
	for i := 0; i < count; i++ {
		svrs = append(svrs, &Server{Weight: i + 1, URL: fmt.Sprintf("192.168.1.%d", i+1)})
	}
	return svrs
}

func readCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestRoundRobinLoadBalancer(t *testing.T) {
	assert := assert.New(t)

	var svrs []*Server
	lb := NewLoadBalancer(&LoadBalanceSpec{Policy: "roundRobin"}, svrs)
	assert.Nil(lb.ChooseServer(nil))

	svrs = prepareServers(10)
	lb = NewLoadBalancer(&LoadBalanceSpec{Policy: "roundRobin"}, svrs)
	for i := 0; i < 10; i++ {
		svr := lb.ChooseServer(nil)
		assert.Equal(svr.Weight, i+1)
	}

	lb = NewLoadBalancer(&LoadBalanceSpec{Policy: "unknow"}, svrs)
	for i := 0; i < 10; i++ {
		svr := lb.ChooseServer(nil)
		assert.Equal(svr.Weight, i+1)
	}
}

func TestRandomLoadBalancer(t *testing.T) {
	assert := assert.New(t)
	rand.Seed(0)

	var svrs []*Server
	lb := NewLoadBalancer(&LoadBalanceSpec{Policy: "random"}, svrs)
	assert.Nil(lb.ChooseServer(nil))

	svrs = prepareServers(10)
	counter := [10]int{}

	lb = NewLoadBalancer(&LoadBalanceSpec{Policy: "random"}, svrs)
	for i := 0; i < 10000; i++ {
		svr := lb.ChooseServer(nil)
		assert.NotNil(svr)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		if v := counter[i]; v < 900 || v > 1100 {
			t.Errorf("possibility is not even with value %v", v)
		}
	}
}

func TestWeightedRandomLoadBalancer(t *testing.T) {
	assert := assert.New(t)
	rand.Seed(0)

	var svrs []*Server
	lb := NewLoadBalancer(&LoadBalanceSpec{Policy: "weightedRandom"}, svrs)
	assert.Nil(lb.ChooseServer(nil))

	svrs = prepareServers(10)
	counter := [10]int{}

	lb = NewLoadBalancer(&LoadBalanceSpec{Policy: "weightedRandom"}, svrs)
	for i := 0; i < 10000; i++ {
		svr := lb.ChooseServer(nil)
		assert.NotNil(svr)
		counter[svr.Weight-1]++
	}

	v := 0
	for i := 0; i < 10; i++ {
		if v >= counter[i] {
			t.Error("possibility is not weighted even")
		}
		v = counter[i]
	}
}

func TestIPHashLoadBalancer(t *testing.T) {
	assert := assert.New(t)

	var svrs []*Server
	lb := NewLoadBalancer(&LoadBalanceSpec{Policy: "ipHash"}, svrs)
	assert.Nil(lb.ChooseServer(nil))

	svrs = prepareServers(10)
	lb = NewLoadBalancer(&LoadBalanceSpec{Policy: "ipHash"}, svrs)

	counter := [10]int{}
	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.Header.Add("X-Real-Ip", fmt.Sprintf("192.168.1.%d", i+1))
		r, _ := httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		assert.GreaterOrEqual(counter[i], 1)
	}
}

func TestHeaderHashLoadBalancer(t *testing.T) {
	assert := assert.New(t)

	var svrs []*Server
	lb := NewLoadBalancer(&LoadBalanceSpec{
		Policy:        "headerHash",
		HeaderHashKey: "X-Header",
	}, svrs)
	assert.Nil(lb.ChooseServer(nil))

	svrs = prepareServers(10)
	lb = NewLoadBalancer(&LoadBalanceSpec{
		Policy:        "headerHash",
		HeaderHashKey: "X-Header",
	}, svrs)

	counter := [10]int{}
	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.Header.Add("X-Header", fmt.Sprintf("abcd-%d", i))
		r, _ := httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		counter[svr.Weight-1]++
	}

	for i := 0; i < 10; i++ {
		assert.GreaterOrEqual(counter[i], 1)
	}
}

func TestStickySession_ConsistentHash(t *testing.T) {
	assert := assert.New(t)

	servers := prepareServers(10)
	lb := NewLoadBalancer(&LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode:          "CookieConsistentHash",
			AppCookieName: "AppCookie",
		},
	}, servers)

	req := &http.Request{Header: http.Header{}}
	req.AddCookie(&http.Cookie{Name: "AppCookie", Value: "abcd-1"})
	r, _ := httpprot.NewRequest(req)
	svr1 := lb.ChooseServer(r)

	for i := 0; i < 100; i++ {
		svr := lb.ChooseServer(r)
		assert.Equal(svr1, svr)
	}
}

func TestStickySession_DurationBased(t *testing.T) {
	assert := assert.New(t)

	servers := prepareServers(10)
	lb := NewLoadBalancer(&LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode: StickySessionModeDurationBased,
		},
	}, servers)

	r, _ := httpprot.NewRequest(&http.Request{Header: http.Header{}})
	svr1 := lb.ChooseServer(r)
	resp, _ := httpprot.NewResponse(&http.Response{Header: http.Header{}})
	lb.ReturnServer(svr1, r, resp)
	c := readCookie(resp.Cookies(), StickySessionDefaultLBCookieName)

	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.AddCookie(&http.Cookie{Name: StickySessionDefaultLBCookieName, Value: c.Value})
		r, _ = httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		assert.Equal(svr1, svr)

		resp, _ = httpprot.NewResponse(&http.Response{Header: http.Header{}})
		lb.ReturnServer(svr, r, resp)
		c = readCookie(resp.Cookies(), StickySessionDefaultLBCookieName)
	}
}

func TestStickySession_ApplicationBased(t *testing.T) {
	assert := assert.New(t)

	servers := prepareServers(10)
	appCookieName := "x-app-cookie"
	lb := NewLoadBalancer(&LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode:          StickySessionModeApplicationBased,
			AppCookieName: appCookieName,
		},
	}, servers)

	r, _ := httpprot.NewRequest(&http.Request{Header: http.Header{}})
	svr1 := lb.ChooseServer(r)
	resp, _ := httpprot.NewResponse(&http.Response{Header: http.Header{}})
	resp.SetCookie(&http.Cookie{Name: appCookieName, Value: ""})
	lb.ReturnServer(svr1, r, resp)
	c := readCookie(resp.Cookies(), StickySessionDefaultLBCookieName)

	for i := 0; i < 100; i++ {
		req := &http.Request{Header: http.Header{}}
		req.AddCookie(&http.Cookie{Name: StickySessionDefaultLBCookieName, Value: c.Value})
		r, _ = httpprot.NewRequest(req)
		svr := lb.ChooseServer(r)
		assert.Equal(svr1, svr)

		resp, _ = httpprot.NewResponse(&http.Response{Header: http.Header{}})
		resp.SetCookie(&http.Cookie{Name: appCookieName, Value: ""})
		lb.ReturnServer(svr, r, resp)
		c = readCookie(resp.Cookies(), StickySessionDefaultLBCookieName)
	}
}

func BenchmarkSign(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sign([]byte("192.168.1.2"))
	}
}

func TestHealthCheck(t *testing.T) {
	assert := assert.New(t)
	servers := prepareServers(3)
	NewLoadBalancer(&LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		HealthCheck: &HealthCheckSpec{
			Interval: 3,
			Fails:    2,
		},
	}, servers)
	for _, svr := range servers {
		assert.True(svr.healthy())
	}

	time.Sleep(5 * time.Second)
	for _, svr := range servers {
		assert.True(svr.healthy())
	}

	time.Sleep(5 * time.Second)
	for _, svr := range servers {
		assert.True(!svr.healthy())
	}
}
