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
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func readCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestStickySession_ConsistentHash(t *testing.T) {
	assert := assert.New(t)

	servers := prepareServers(10)
	spec := &LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode:          "CookieConsistentHash",
			AppCookieName: "AppCookie",
		},
	}

	lb := NewGeneralLoadBalancer(spec, servers)
	lb.Init(NewHTTPSessionSticker, nil, nil)

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
	spec := &LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode: StickySessionModeDurationBased,
		},
	}

	lb := NewGeneralLoadBalancer(spec, servers)
	lb.Init(NewHTTPSessionSticker, nil, nil)

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
	spec := &LoadBalanceSpec{
		Policy: LoadBalancePolicyRandom,
		StickySession: &StickySessionSpec{
			Mode:          StickySessionModeApplicationBased,
			AppCookieName: appCookieName,
		},
	}
	lb := NewGeneralLoadBalancer(spec, servers)
	lb.Init(NewHTTPSessionSticker, nil, nil)

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
