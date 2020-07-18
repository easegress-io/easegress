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
			spec := &PoolSpec{
				ServersTags: tt.fields.serversTags,
				Servers:     tt.fields.servers,
			}
			got := newServers(spec).servers
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
