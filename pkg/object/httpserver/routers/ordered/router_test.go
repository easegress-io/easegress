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

package ordered

import (
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestCreateInstance(t *testing.T) {
	rules := routers.Rules{
		&routers.Rule{
			Host:       "www.megaease.com",
			HostRegexp: `^[^.]+\.megaease\.com$`,
			IPFilterSpec: &ipfilter.Spec{
				AllowIPs: []string{"192.168.1.0/24"},
			},
			Paths: []*routers.Path{
				{
					Path: "/api/test",
				},
				{
					PathRegexp: `^test$`,
				},
			},
		},
		&routers.Rule{
			Host:       "google.com",
			HostRegexp: `^[^.]+\.google\.com$`,
			IPFilterSpec: &ipfilter.Spec{
				AllowIPs: []string{"192.168.1.0/24"},
			},
			Paths: []*routers.Path{
				{
					Path: "/api/google",
				},
				{
					PathRegexp: `^google$`,
				},
			},
		},
	}

	rules.Init()
	var router *orderedRouter = kind.CreateInstance(rules).(*orderedRouter)

	assert := assert.New(t)
	assert.Equal(len(rules), len(router.rules))

	for i := 0; i < len(rules); i++ {
		rule := rules[i]
		muxRule := router.rules[i]
		assert.Equal(len(rule.Paths), len(muxRule.paths))

		for j := 0; j < len(rule.Paths); j++ {
			path := rule.Paths[j]
			route := muxRule.paths[j]

			if path.PathRegexp != "" {
				assert.NotNil(route.pathRE)
			}

		}
	}
}

func TestMuxRoutePathMatch(t *testing.T) {
	assert := assert.New(t)
	path := "/abc"
	// stdr, _ := http.NewRequest(http.MethodGet, path, nil)
	// req, _ := httpprot.NewRequest(stdr)

	// 1. match path
	p := &routers.Path{}
	p.Init(nil)
	mp := newMuxPath(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// exact match
	p = &routers.Path{Path: "/abc"}
	p.Init(nil)
	mp = newMuxPath(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// prefix
	p = &routers.Path{PathPrefix: "/ab"}
	p.Init(nil)
	mp = newMuxPath(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// regexp
	p = &routers.Path{PathRegexp: "/[a-z]+"}
	p.Init(nil)
	mp = newMuxPath(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// invalid regexp
	p = &routers.Path{PathRegexp: "/[a-z+"}
	p.Init(nil)
	mp = newMuxPath(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// not match
	p = &routers.Path{Path: "/xyz"}
	p.Init(nil)
	mp = newMuxPath(p)
	assert.NotNil(mp)
	assert.False(mp.matchPath(path))
}

func TestSearch(t *testing.T) {
	assert := assert.New(t)

	rules := routers.Rules{
		&routers.Rule{
			Host: "www.megaease.com",
			IPFilterSpec: &ipfilter.Spec{
				BlockByDefault: true,
				AllowIPs:       []string{"192.168.1.0/24"},
			},
			Paths: []*routers.Path{
				{
					Path: "/api/test",
				},
				{
					PathRegexp: `^test$`,
				},
			},
		},
	}

	rules.Init()
	var router *orderedRouter = kind.CreateInstance(rules).(*orderedRouter)

	tests := []struct {
		host         string
		ip           string
		path         string
		result       bool
		iPNotAllowed bool
	}{
		{
			host: "1233434.com",
			path: "test",
		},
		{
			host:         "www.megaease.com",
			ip:           "10.168.1.0",
			iPNotAllowed: true,
			path:         "test",
		},
		{
			host: "www.megaease.com",
			ip:   "192.168.1.1",
			path: "abc",
		},
		{
			host:   "www.megaease.com",
			ip:     "192.168.1.1",
			path:   "/api/test",
			result: true,
		},
		{
			host:   "www.megaease.com",
			ip:     "192.168.1.1",
			path:   "test",
			result: true,
		},
	}

	for _, test := range tests {
		headers := map[string][]string{
			"X-Real-Ip": {test.ip},
		}
		stdr, _ := http.NewRequest("GET", test.path, nil)
		stdr.Host = test.host
		stdr.Header = headers
		req, _ := httpprot.NewRequest(stdr)
		ctx := routers.NewContext(req)

		router.Search(ctx)

		assert.Equal(test.result, ctx.Route != nil)
		assert.Equal(test.iPNotAllowed, ctx.IPMismatch)

	}
}

func TestRewrite(t *testing.T) {
	assert := assert.New(t)

	t.Run("no rewrite", func(t *testing.T) {
		mp := newMuxPath(
			&routers.Path{
				RewriteTarget: "",
			})

		stdr, _ := http.NewRequest("GET", "https://www.megaease.com/foo?a=1&b=2", nil)
		req, _ := httpprot.NewRequest(stdr)
		ctx := routers.NewContext(req)

		mp.Rewrite(ctx)
		assert.Equal("/foo", req.Path())
	})

	t.Run("rewrite by exact match", func(t *testing.T) {
		mp := newMuxPath(
			&routers.Path{
				Path:          "/foo",
				RewriteTarget: "/bar",
			})

		stdr, _ := http.NewRequest("GET", "https://www.megaease.com/foo?a=1&b=2", nil)
		req, _ := httpprot.NewRequest(stdr)
		ctx := routers.NewContext(req)
		mp.Rewrite(ctx)
		assert.Equal("/bar", req.Path())
	})

	t.Run("rewrite by prefix", func(t *testing.T) {
		mp := newMuxPath(
			&routers.Path{
				PathPrefix:    "/fo",
				RewriteTarget: "/ba",
			})

		stdr, _ := http.NewRequest("GET", "https://www.megaease.com/foo?a=1&b=2", nil)
		req, _ := httpprot.NewRequest(stdr)
		ctx := routers.NewContext(req)
		mp.Rewrite(ctx)
		assert.Equal("/bao", req.Path())
	})

	t.Run("rewrite by regexp", func(t *testing.T) {
		mp := newMuxPath(
			&routers.Path{
				PathRegexp:    "/(fo).",
				RewriteTarget: "/ba$1",
			})

		stdr, _ := http.NewRequest("GET", "https://www.megaease.com/foo?a=1&b=2", nil)
		req, _ := httpprot.NewRequest(stdr)
		ctx := routers.NewContext(req)
		mp.Rewrite(ctx)
		assert.Equal("/bafo", req.Path())
	})
}
