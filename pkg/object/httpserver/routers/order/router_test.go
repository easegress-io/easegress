package order

import (
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/ipfilter"
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

	rules.Init(nil)
	var router *OrderRouter = kind.CreateInstance(rules).(*OrderRouter)

	assert := assert.New(t)
	assert.Equal(len(rules), len(router.rules))

	for i := 0; i < len(rules); i++ {
		rule := rules[i]
		muxRule := router.rules[i]
		assert.Equal(len(rule.Paths), len(muxRule.routes))

		for j := 0; j < len(rule.Paths); j++ {
			path := rule.Paths[j]
			route := muxRule.routes[j]

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
	mp := newRoute(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// exact match
	p = &routers.Path{Path: "/abc"}
	p.Init(nil)
	mp = newRoute(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// prefix
	p = &routers.Path{PathPrefix: "/ab"}
	p.Init(nil)
	mp = newRoute(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// regexp
	p = &routers.Path{PathRegexp: "/[a-z]+"}
	p.Init(nil)
	mp = newRoute(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// invalid regexp
	p = &routers.Path{PathRegexp: "/[a-z+"}
	p.Init(nil)
	mp = newRoute(p)
	assert.NotNil(mp)
	assert.True(mp.matchPath(path))

	// not match
	p = &routers.Path{Path: "/xyz"}
	p.Init(nil)
	mp = newRoute(p)
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

	rules.Init(nil)
	var router *OrderRouter = kind.CreateInstance(rules).(*OrderRouter)

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
		assert.Equal(test.iPNotAllowed, ctx.IPNotAllowed)

	}
}
