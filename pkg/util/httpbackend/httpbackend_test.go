package httpbackend

import (
	"reflect"
	"testing"
)

func TestPickServers(t *testing.T) {
	type fields struct {
		ServersTags []string
		Servers     []Server
	}
	tests := []struct {
		name   string
		fields fields
		want   []Server
	}{
		{
			name: "pickNone",
			fields: fields{
				ServersTags: []string{"v2"},
				Servers: []Server{
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
			want: []Server{},
		},
		{
			name: "pickSome",
			fields: fields{
				ServersTags: []string{"v1"},
				Servers: []Server{
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
			want: []Server{
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"v1"},
				},
			},
		},
		{
			name: "pickAll1",
			fields: fields{
				ServersTags: nil,
				Servers: []Server{
					{URL: "http://127.0.0.1:9090"},
					{URL: "http://127.0.0.1:9090"},
					{URL: "http://127.0.0.1:9090"},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"green"},
					},
				},
			},
			want: []Server{
				{URL: "http://127.0.0.1:9090"},
				{URL: "http://127.0.0.1:9090"},
				{URL: "http://127.0.0.1:9090"},
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"green"},
				},
			},
		},
		{
			name: "pickAll2",
			fields: fields{
				ServersTags: []string{"v1"},
				Servers: []Server{
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1", "v3"},
					},
				},
			},
			want: []Server{
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"v1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"v1"},
				},
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"v1", "v3"},
				},
			},
		},
		{
			name: "pickMultiTags",
			fields: fields{
				ServersTags: []string{"v1", "green"},
				Servers: []Server{
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"d1", "v1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1", "d1", "green"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"green", "d1", "v1"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1"},
					},
					{
						URL:  "http://127.0.0.1:9090",
						Tags: []string{"v1", "v3"},
					},
				},
			},
			want: []Server{
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"d1", "v1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"v1", "d1", "green"},
				},
				{
					URL:  "http://127.0.0.1:9090",
					Tags: []string{"green", "d1", "v1"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Spec{
				ServersTags: tt.fields.ServersTags,
				Servers:     tt.fields.Servers,
			}
			if got := s.pickServers(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Spec.pickServers() = %v, want %v", got, tt.want)
			}
		})
	}
}
