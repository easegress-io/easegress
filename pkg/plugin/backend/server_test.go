package backend

import (
	"reflect"
	"testing"
)

func TestPickservers(t *testing.T) {
	type fields struct {
		serversTags []string
		servers     []*server
	}
	tests := []struct {
		name   string
		fields fields
		want   []*server
	}{
		{
			name: "pickNone",
			fields: fields{
				serversTags: []string{"v2"},
				servers: []*server{
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
			want: []*server{},
		},
		{
			name: "pickSome",
			fields: fields{
				serversTags: []string{"v1"},
				servers: []*server{
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
			want: []*server{
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
				servers: []*server{
					{URL: "http://127.0.0.1:9091", Weight: 33},
					{URL: "http://127.0.0.1:9092", Weight: 1},
					{URL: "http://127.0.0.1:9093", Weight: 0},
					{
						URL:  "http://127.0.0.1:9094",
						Tags: []string{"green"},
					},
				},
			},
			want: []*server{
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
				servers: []*server{
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
			want: []*server{
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
				servers: []*server{
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
			want: []*server{
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
			spec := &poolSpec{
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
