package order

import (
	"testing"

	"github.com/megaease/easegress/pkg/object/httpserver/routers"
	"github.com/stretchr/testify/assert"
)

func TestMuxRoutePathMatch(t *testing.T) {
	assert := assert.New(t)
	path := "http://www.megaease.com/abc"
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

	// // 2. match method
	// p = &routers.Path{}
	// p.Init(nil)
	// mp = newRoute(p)
	// mp = newRoute(nil, &routers.Path{})
	// assert.NotNil(mp)
	// assert.True(mp.matchMethod(req))

	// mp = newRoute(nil, &routers.Path{Methods: []string{http.MethodGet}})
	// assert.NotNil(mp)
	// assert.True(mp.matchMethod(req))

	// mp = newRoute(nil, &routers.Path{Methods: []string{http.MethodPut}})
	// assert.NotNil(mp)
	// assert.False(mp.matchMethod(req))

	// // 3. match headers
	// stdr.Header.Set("X-Test", "test1")

	// mp = newRoute(nil, &routers.Path{Headers: []*routers.Header{{
	// 	Key:    "X-Test",
	// 	Values: []string{"test1", "test2"},
	// }}})
	// assert.True(mp.matchHeaders(req))

	// mp = newRoute(nil, &routers.Path{Headers: []*routers.Header{{
	// 	Key:    "X-Test",
	// 	Regexp: "test[0-9]",
	// }}})
	// assert.True(mp.matchHeaders(req))

	// mp = newRoute(nil, &routers.Path{Headers: []*routers.Header{{
	// 	Key:    "X-Test2",
	// 	Values: []string{"test1", "test2"},
	// }}})
	// assert.False(mp.matchHeaders(req))

	// // 4. rewrite
	// mp = newRoute(nil, &routers.Path{Path: "/abc"})
	// assert.NotNil(mp)
	// mp.rewrite(req)
	// assert.Equal("/abc", req.Path())

	// mp = newRoute(nil, &routers.Path{Path: "/abc", RewriteTarget: "/xyz"})
	// assert.NotNil(mp)
	// mp.rewrite(req)
	// assert.Equal("/xyz", req.Path())

	// mp = newRoute(nil, &routers.Path{PathPrefix: "/xy", RewriteTarget: "/ab"})
	// assert.NotNil(mp)
	// mp.rewrite(req)
	// assert.Equal("/abz", req.Path())

	// mp = newRoute(nil, &routers.Path{PathRegexp: "/([a-z]+)", RewriteTarget: "/1$1"})
	// assert.NotNil(mp)
	// mp.rewrite(req)
	// assert.Equal("/1abz", req.Path())

	// // 5. match query
	// stdr.URL.RawQuery = "q=v1&q=v2"
	// mp = newRoute(nil, &routers.Path{Queries: []*routers.Query{{
	// 	Key:    "q",
	// 	Values: []string{"v1", "v2"},
	// }}})
	// assert.True(mp.matchQueries(req))

	// mp = newRoute(nil, &routers.Path{Queries: []*routers.Query{{
	// 	Key:    "q",
	// 	Regexp: "v[0-9]",
	// }}})
	// assert.True(mp.matchQueries(req))

	// mp = newRoute(nil, &routers.Path{Queries: []*routers.Query{{
	// 	Key:    "q2",
	// 	Values: []string{"v1", "v2"},
	// }}})
	// assert.False(mp.matchQueries(req))
}
