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

package backend

import (
	"reflect"
	"testing"
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
				LoadBalance{Policy: PolicyRoundRobin})
			got := ss.servers
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
