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
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/util/hashtool"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

func TestPickservers(t *testing.T) {
	type fields struct {
		serversTags []string
		servers     []*Server
	}
	tests := []struct {
		name   string
		fields fields
		want   []*Server
	}{
		{
			name: "pickNone",
			fields: fields{
				serversTags: []string{"v2"},
				servers: []*Server{
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"green"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v3"},
					},
				},
			},
			want: []*Server{},
		},
		{
			name: "pickSome",
			fields: fields{
				serversTags: []string{"v1"},
				servers: []*Server{
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"green"},
					},
					{
						URL:  "http://127.0.0.1:9091",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9092",
						Tags: []string{"v3"},
					},
				},
			},
			want: []*Server{
				{
					URL:  "http://127.0.0.1:9091",
					Tags: []string{"v1"},
				},
			},
		},
		{
			name: "pickAll1",
			fields: fields{
				serversTags: nil,
				servers: []*Server{
					{URL: "http://127.0.0.1:9091", Weight: 33},
					{URL: "http://127.0.0.1:9092", Weight: 1},
					{URL: "http://127.0.0.1:9093", Weight: 0},
					{
						URL:  "http://127.0.0.1:9094",
						Tags: []string{"green"},
					},
				},
			},
			want: []*Server{
				{URL: "http://127.0.0.1:9091", Weight: 33},
				{URL: "http://127.0.0.1:9092", Weight: 1},
				{URL: "http://127.0.0.1:9093", Weight: 0},
				{
					URL:  "http://127.0.0.1:9094",
					Tags: []string{"green"},
				},
			},
		},
		{
			name: "pickAll2",
			fields: fields{
				serversTags: []string{"v1"},
				servers: []*Server{
					{
						URL:  "http://127.0.0.1:9091",
						Tags: []string{"v1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9092",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9093",
						Tags: []string{"v1", "v3"},
					},
				},
			},
			want: []*Server{
				{
					URL:  "http://127.0.0.1:9091",
					Tags: []string{"v1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9092",
					Tags: []string{"v1"},
				},
				{
					URL:  "http://127.0.0.1:9093",
					Tags: []string{"v1", "v3"},
				},
			},
		},
		{
			name: "pickMultiTags",
			fields: fields{
				serversTags: []string{"v1", "green"},
				servers: []*Server{
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"d1", "v1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9091",
						Tags: []string{"v1", "d1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9092",
						Tags: []string{"green", "d1", "v1"},
					},
					{
						URL:  "http://127.0.0.1:9093",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9094",
						Tags: []string{"v1", "v3"},
					},
				},
			},
			want: []*Server{
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"d1", "v1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9091",
					Tags: []string{"v1", "d1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9092",
					Tags: []string{"green", "d1", "v1"},
				},
				{
					URL:  "http://127.0.0.1:9093",
					Tags: []string{"v1"},
				},
				{
					URL:  "http://127.0.0.1:9094",
					Tags: []string{"v1", "v3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := newStaticServers(tt.fields.servers,
				tt.fields.serversTags,
				&LoadBalance{Policy: PolicyRoundRobin})
			got := ss.servers
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStaticServers(t *testing.T) {
	servers := []*Server{
		{
			URL:    "http://127.0.0.1:9090",
			Tags:   []string{"d1", "v1", "green"},
			Weight: 1,
		},
		{
			URL:    "http://127.0.0.1:9091",
			Tags:   []string{"v1", "d1", "green"},
			Weight: 2,
		},
		{
			URL:    "http://127.0.0.1:9092",
			Tags:   []string{"green", "d1", "v1"},
			Weight: 3,
		},
		{
			URL:    "http://127.0.0.1:9093",
			Tags:   []string{"v1"},
			Weight: 4,
		},
		{
			URL:    "http://127.0.0.1:9094",
			Tags:   []string{"v1", "v3"},
			Weight: 5,
		},
	}

	for _, s := range servers {
		str := s.String()
		if str != fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight) {
			t.Error("s.String returns unexpected result")
		}
	}

	ss := newStaticServers(servers, []string{}, nil)
	if ss.len() != len(servers) {
		t.Errorf("ss.len() is not %d", len(servers))
	}

	ctx := &contexttest.MockedHTTPContext{}
	for i := 0; i < len(servers); i++ {
		if ss.next(ctx) != servers[i] {
			t.Errorf("ss.next() returns unexpected server")
		}
	}

	ss.lb.Policy = "badPolicy"
	for i := 0; i < len(servers); i++ {
		if ss.next(ctx) != servers[i] {
			t.Errorf("ss.next() returns unexpected server")
		}
	}

	ss.lb.Policy = PolicyRandom
	for i := 0; i < len(servers); i++ {
		ss.next(ctx)
	}

	ss.lb.Policy = PolicyWeightedRandom
	for i := 0; i < len(servers); i++ {
		ss.next(ctx)
	}

	ip := ""
	ctx.MockedRequest.MockedRealIP = func() string {
		return ip
	}
	ss.lb.Policy = PolicyIPHash
	for i := 0; i < len(servers)*5; i++ {
		ip = fmt.Sprintf("111.222.111.%d", i)
		sum32 := int(hashtool.Hash32(ip))
		s := servers[sum32%len(servers)]
		if ss.next(ctx) != s {
			t.Errorf("ss.next() returns unexpected server")
		}
	}

	header := http.Header{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	ss.lb.Policy = PolicyHeaderHash
	if ss.lb.Validate() == nil {
		t.Error("LoadBalance.Validate should fail")
	}

	ss.lb.HeaderHashKey = "X-Megaease"
	if ss.lb.Validate() != nil {
		t.Error("LoadBalance.Validate should succeed")
	}
	for i := 0; i < len(servers)*5; i++ {
		v := fmt.Sprintf("value-%d", i)
		header.Set(ss.lb.HeaderHashKey, v)
		sum32 := int(hashtool.Hash32(v))
		s := servers[sum32%len(servers)]
		if ss.next(ctx) != s {
			t.Errorf("ss.next() returns unexpected server")
		}
	}
}

func TestServers(t *testing.T) {
	servers := []*Server{
		{
			URL:    "http://127.0.0.1:9090",
			Tags:   []string{"d1", "v1", "green"},
			Weight: 1,
		},
		{
			URL:    "http://127.0.0.1:9091",
			Tags:   []string{"v1", "d1", "green"},
			Weight: 2,
		},
		{
			URL:    "http://127.0.0.1:9092",
			Tags:   []string{"green", "d1", "v1"},
			Weight: 3,
		},
		{
			URL:    "http://127.0.0.1:9093",
			Tags:   []string{"v1"},
			Weight: 4,
		},
		{
			URL:    "http://127.0.0.1:9094",
			Tags:   []string{"v1", "v3"},
			Weight: 5,
		},
	}
	ps := &PoolSpec{
		ServersTags:     []string{},
		ServiceRegistry: "service registry",
		ServiceName:     "service name",
		Servers:         []*Server{},
	}

	ctx := &contexttest.MockedHTTPContext{}

	s := newServers(ps)
	if s.len() != 0 {
		t.Errorf("servers.len() should be 0")
	}
	s.close()

	ps = &PoolSpec{
		ServersTags:     []string{},
		ServiceRegistry: "service registry",
		ServiceName:     "service name",
		Servers:         servers,
	}
	s = newServers(ps)
	if s.len() != len(servers) {
		t.Errorf("servers.len() is not %d", len(servers))
	}
	for i := 0; i < len(servers); i++ {
		if svr, e := s.next(ctx); e != nil || svr != servers[i] {
			t.Errorf("ss.next() returns unexpected server")
		}
	}
	s.close()

	ps = &PoolSpec{
		ServersTags:     []string{},
		ServiceRegistry: "service registry",
		Servers:         []*Server{},
		ServiceName:     "testservice",
	}
	fnGetService.Store(func(serviceRegistry, serviceName string) (*serviceregistry.Service, error) {
		return nil, fmt.Errorf("dummy error")
	})
	s = newServers(ps)
	if s.len() != 0 {
		t.Errorf("servers.len() should be 0")
	}
	s.close()

	svcservers := []*serviceregistry.Server{
		{
			ServiceName: "testservice",
			Hostname:    "server1",
			HostIP:      "192.168.1.1",
			Port:        80,
		},
		{
			ServiceName: "testservice",
			Hostname:    "server2",
			HostIP:      "192.168.1.2",
			Port:        80,
		},
		{
			ServiceName: "testservice",
			Hostname:    "server3",
			HostIP:      "192.168.1.3",
			Port:        80,
		},
	}
	service, _ := serviceregistry.NewService("testservice", svcservers)

	fnGetService.Store(func(serviceRegistry, serviceName string) (*serviceregistry.Service, error) {
		return service, nil
	})
	s = newServers(ps)
	if s.len() != len(service.Servers()) {
		t.Errorf("servers.len() is not %d", len(service.Servers()))
	}

	svcservers = append(svcservers, &serviceregistry.Server{
		ServiceName: "testservice",
		Hostname:    "server4",
		HostIP:      "192.168.1.4",
		Port:        80,
	})
	service.Update(svcservers)

	time.Sleep(100 * time.Millisecond)
	if s.len() != len(service.Servers()) {
		t.Errorf("servers.len() is not %d", len(service.Servers()))
	}

	service.Close("close")
	s.close()
}
