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

package httpproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/stretchr/testify/assert"
)

func TestHTTPHealthCheckSpec(t *testing.T) {
	testCases := []struct {
		spec   *ProxyHealthCheckSpec
		failed bool
		msg    string
	}{
		{
			spec:   &ProxyHealthCheckSpec{},
			failed: false,
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					URI: "::::::::",
				},
			},
			failed: true,
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Method: "not-a-method",
				},
			},
			failed: true,
			msg:    "invalid method",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Username: "123",
				},
			},
			failed: true,
			msg:    "empty password",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Password: "123",
				},
			},
			failed: true,
			msg:    "empty username",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Port: -100,
				},
			},
			failed: true,
			msg:    "invalid port",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						StatusCodes: [][]int{{2, 1}},
					},
				},
			},
			failed: true,
			msg:    "invalid status code range",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						StatusCodes: [][]int{{1, 2, 3}},
					},
				},
			},
			failed: true,
			msg:    "invalid status code range",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: ""},
						},
					},
				},
			},
			failed: true,
			msg:    "empty match header name",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: "X-Test", Type: "not-a-type"},
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match header type",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: "X-Test", Type: "regexp", Value: "["},
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match header regex",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						Body: &HealthCheckBodyMatch{
							Type:  "not-a-type",
							Value: "123",
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match body type",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Match: &HealthCheckMatch{
						Body: &HealthCheckBodyMatch{
							Type:  "regexp",
							Value: "[",
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match body regex",
		},
		{
			spec: &ProxyHealthCheckSpec{
				HTTPHealthCheckSpec: HTTPHealthCheckSpec{
					Port:   10080,
					URI:    "/healthz",
					Method: "GET",
					Headers: map[string]string{
						"X-Test": "easegress",
					},
					Body:     "easegress",
					Username: "admin",
					Password: "test",
					Match: &HealthCheckMatch{
						StatusCodes: [][]int{{200, 300}},
						Headers: []HealthCheckHeaderMatch{
							{Name: "X-Test", Type: "exact", Value: "easegress"},
							{Name: "X-Test-Re", Type: "regexp", Value: ".*"},
						},
						Body: &HealthCheckBodyMatch{
							Value: ".*",
							Type:  "regexp",
						},
					},
				},
			},
			failed: false,
		},
	}
	for _, tc := range testCases {
		err := tc.spec.Validate()
		if tc.failed {
			assert.NotNil(t, err, tc)
			assert.Contains(t, err.Error(), tc.msg, tc)
		} else {
			assert.Nil(t, err, tc)
		}
	}

	hc := NewHTTPHealthChecker(nil, &ProxyHealthCheckSpec{}).(*httpHealthChecker)
	expected := &ProxyHealthCheckSpec{
		HTTPHealthCheckSpec: HTTPHealthCheckSpec{
			Method: "GET",
			Match: &HealthCheckMatch{
				StatusCodes: [][]int{{200, 399}},
			},
		},
	}
	assert.Equal(t, expected, hc.spec, "default spec")
}

func startServer(port int, handler http.Handler, checkReq func() *http.Request) (*http.Server, error) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: handler,
	}
	go server.ListenAndServe()
	for i := 0; i < 100; i++ {
		req := checkReq()
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return server, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("failed to start server")
}

func TestHTTPHealthCheck(t *testing.T) {
	assert := assert.New(t)

	handleFunc := atomic.Value{}
	handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {})
	mux := http.ServeMux{}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		handleFunc.Load().(func(w http.ResponseWriter, r *http.Request))(w, r)
	})
	server, err := startServer(10080, &mux, func() *http.Request {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:10080/healthz", nil)
		return req
	})
	assert.Nil(err)
	defer server.Shutdown(context.Background())

	type respConfig struct {
		header map[string]string
		status int
		body   string
	}

	respHandler := func(w http.ResponseWriter, cfg respConfig) {
		for k, v := range cfg.header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(cfg.status)
		w.Write([]byte(cfg.body))
	}

	{
		spec := &ProxyHealthCheckSpec{
			HTTPHealthCheckSpec: HTTPHealthCheckSpec{
				Port:   10080,
				URI:    "/healthz?test=easegress",
				Method: http.MethodPost,
				Headers: map[string]string{
					"X-Test": "easegress",
				},
				Body:     "easegress",
				Username: "admin",
				Password: "test",
				Match: &HealthCheckMatch{
					StatusCodes: [][]int{{200, 299}, {400, 499}},
					Headers: []HealthCheckHeaderMatch{
						{Name: "H-One", Type: "exact", Value: "V-One"},
						{Name: "H-Prefix", Type: "regexp", Value: "^V-"},
					},
					Body: &HealthCheckBodyMatch{
						Value: "success",
						Type:  "contains",
					},
				},
			},
		}
		succConfig := func() respConfig {
			return respConfig{
				header: map[string]string{
					"H-One":    "V-One",
					"H-Prefix": "V-Two",
				},
				status: http.StatusOK,
				body:   "success",
			}
		}

		hc := NewHTTPHealthChecker(nil, spec).(*httpHealthChecker)
		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal("POST", r.Method)
			assert.Equal("/healthz", r.URL.Path)
			assert.Equal("test=easegress", r.URL.RawQuery)
			assert.Equal("easegress", r.Header.Get("X-Test"))

			username, password, ok := r.BasicAuth()
			assert.True(ok)
			assert.Equal("admin", username)
			assert.Equal("test", password)

			body, err := io.ReadAll(r.Body)
			assert.Nil(err)
			assert.Equal("easegress", string(body))

			respHandler(w, succConfig())
		})
		s := &proxies.Server{URL: "http://127.0.0.1:8080?test=megaease"}
		assert.True(hc.Check(s))

		// wrong status code
		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			cfg := succConfig()
			cfg.status = http.StatusFound
			respHandler(w, cfg)
		})
		assert.False(hc.Check(s))

		// wrong header
		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			cfg := succConfig()
			cfg.header["H-One"] = "V-Two"
			respHandler(w, cfg)
		})
		assert.False(hc.Check(s))

		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			cfg := succConfig()
			cfg.header["H-Prefix"] = "wrong"
			respHandler(w, cfg)
		})
		assert.False(hc.Check(s))

		// wrong body
		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			cfg := succConfig()
			cfg.body = "failed!!!"
			respHandler(w, cfg)
		})
		assert.False(hc.Check(s))

		handleFunc.Store(func(w http.ResponseWriter, r *http.Request) {
			cfg := succConfig()
			cfg.status = http.StatusNotFound
			cfg.body = "big success"
			respHandler(w, cfg)
		})
		assert.True(hc.Check(s))
	}
}

func TestWebSocketHealthCheckSpec(t *testing.T) {
	testCases := []struct {
		spec   *WSProxyHealthCheckSpec
		failed bool
		msg    string
	}{
		{
			spec:   &WSProxyHealthCheckSpec{},
			failed: true,
			msg:    "empty health check spec",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				HTTP: &HTTPHealthCheckSpec{},
				WS:   &WSHealthCheckSpec{},
			},
			failed: false,
		},
		{
			spec: &WSProxyHealthCheckSpec{
				HTTP: &HTTPHealthCheckSpec{
					Method: "not-a-method",
				},
			},
			failed: true,
			msg:    "invalid method",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Port: -100,
				},
			},
			failed: true,
			msg:    "invalid port",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					URI: "::::::",
				},
			},
			failed: true,
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						StatusCodes: [][]int{{2, 1}},
					},
				},
			},
			failed: true,
			msg:    "invalid status code range",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						StatusCodes: [][]int{{1, 2, 3}},
					},
				},
			},
			failed: true,
			msg:    "invalid status code range",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: ""},
						},
					},
				},
			},
			failed: true,
			msg:    "empty match header name",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: "X-Test", Type: "not-a-type"},
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match header type",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						Headers: []HealthCheckHeaderMatch{
							{Name: "X-Test", Type: "regexp", Value: "["},
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match header regex",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						Body: &HealthCheckBodyMatch{
							Type:  "not-a-type",
							Value: "123",
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match body type",
		},
		{
			spec: &WSProxyHealthCheckSpec{
				WS: &WSHealthCheckSpec{
					Match: &HealthCheckMatch{
						Body: &HealthCheckBodyMatch{
							Type:  "regexp",
							Value: "[",
						},
					},
				},
			},
			failed: true,
			msg:    "invalid match body regex",
		},
	}
	for _, tc := range testCases {
		err := tc.spec.Validate()
		if tc.failed {
			assert.NotNil(t, err, tc)
			assert.Contains(t, err.Error(), tc.msg, tc)
		} else {
			assert.Nil(t, err, tc)
		}
	}

	hc := NewWebSocketHealthChecker(&WSProxyHealthCheckSpec{
		WS: &WSHealthCheckSpec{},
	}).(*wsHealthChecker)

	expected := &WSProxyHealthCheckSpec{
		WS: &WSHealthCheckSpec{
			Match: &HealthCheckMatch{
				StatusCodes: [][]int{{101, 101}},
			},
		},
	}
	assert.Equal(t, expected, hc.spec, "default spec")
}

func TestWebSocketHealthCheck(t *testing.T) {
	assert := assert.New(t)

	mux := http.ServeMux{}

	httpHandler := atomic.Value{}
	httpHandler.Store(func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpHandler.Load().(func(w http.ResponseWriter, r *http.Request))(w, r)
	})

	wsHandler := atomic.Value{}
	wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler.Load().(func(w http.ResponseWriter, r *http.Request))(w, r)
	})

	server, err := startServer(10080, &mux, func() *http.Request {
		req, _ := http.NewRequest("GET", "http://127.0.0.1:10080/healthz", nil)
		return req
	})
	assert.Nil(err)
	defer server.Shutdown(context.Background())

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	{
		spec := &WSProxyHealthCheckSpec{
			HTTP: &HTTPHealthCheckSpec{
				Port: 10080,
				URI:  "/healthz",
				Match: &HealthCheckMatch{
					StatusCodes: [][]int{{200, 299}, {400, 499}},
				},
			},
			WS: &WSHealthCheckSpec{
				Port: 10080,
				URI:  "/ws",
				Headers: map[string]string{
					"X-Test": "easegress",
				},
				Match: &HealthCheckMatch{
					Headers: []HealthCheckHeaderMatch{
						{Name: "H-One", Type: "exact", Value: "V-One"},
						{Name: "H-Prefix", Type: "regexp", Value: "^V-"},
					},
				},
			},
		}

		hc := NewWebSocketHealthChecker(spec).(*wsHealthChecker)

		httpHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal("easegress", r.Header.Get("X-Test"))
			header := http.Header{}
			header.Set("H-One", "V-One")
			header.Set("H-Prefix", "V-Two")
			conn, err := upgrader.Upgrade(w, r, header)
			assert.Nil(err)
			defer conn.Close()
		})
		s := &proxies.Server{URL: "http://127.0.0.1:8080?test=megaease"}
		assert.True(hc.Check(s))

		s = &proxies.Server{URL: "ws://127.0.0.1:8080?test=megaease"}
		assert.True(hc.Check(s))

		// wrong http status code
		httpHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusFound)
		})
		assert.False(hc.Check(s))

		// wrong websocket header
		httpHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			assert.Nil(err)
			defer conn.Close()
		})
		assert.False(hc.Check(s))

		// wrong websocket status code
		wsHandler.Store(func(w http.ResponseWriter, r *http.Request) {
			header := http.Header{}
			header.Set("H-One", "V-One")
			header.Set("H-Prefix", "V-Two")
			w.WriteHeader(http.StatusOK)
		})
		assert.False(hc.Check(s))
	}
}
