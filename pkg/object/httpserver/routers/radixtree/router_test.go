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

package radixtree

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

// art-router implementation below is a based on the original work by
// go-chi in https://github.com/go-chi/chi/blob/master/tree.go
// (MIT licensed). It's been heavily modified for use as a HTTP router.

func TestTree(t *testing.T) {
	hStub := "hStub"
	hIndex := "hIndex"
	hFavicon := "hFavicon"
	hArticleList := "hArticleList"
	hArticleNear := "hArticleNear"
	hArticleShow := "hArticleShow"
	hArticleShowRelated := "hArticleShowRelated"
	hArticleShowOpts := "hArticleShowOpts"
	hArticleSlug := "hArticleSlug"
	hArticleByUser := "hArticleByUser"
	hUserList := "hUserList"
	hUserShow := "hUserShow"
	hAdminCatchall := "hAdminCatchall"
	hAdminAppShow := "hAdminAppShow"
	hAdminAppShowCatchall := "hAdminAppShowCatchall"
	hUserProfile := "hUserProfile"
	hUserSuper := "hUserSuper"
	hUserAll := "hUserAll"
	hHubView1 := "hHubView1"
	hHubView2 := "hHubView2"
	hHubView3 := "hHubView3"

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/",
					Methods: []string{"GET"},
					Backend: hIndex,
				},

				{
					Path:    "/favicon.ico",
					Methods: []string{"GET"},
					Backend: hFavicon,
				},

				{
					Path:    "/pages/*",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/article",
					Methods: []string{"GET"},
					Backend: hArticleList,
				},

				{
					Path:    "/article/",
					Methods: []string{"GET"},
					Backend: hArticleList,
				},

				{
					Path:    "/article/near",
					Methods: []string{"GET"},
					Backend: hArticleNear,
				},

				{
					Path:    "/article/{id}",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/article/{id}",
					Methods: []string{"GET"},
					Backend: hArticleShow,
				},

				{
					Path:    "/article/{id}",
					Methods: []string{"GET"},
					Backend: hArticleShow,
				},

				{
					Path:    "/article/@{user}",
					Methods: []string{"GET"},
					Backend: hArticleByUser,
				},

				{
					Path:    "/article/{sup}/{opts}",
					Methods: []string{"GET"},
					Backend: hArticleShowOpts,
				},

				{
					Path:    "/article/{id}/{opts}",
					Methods: []string{"GET"},
					Backend: hArticleShowOpts,
				},

				{
					Path:    "/article/{iffd}/edit",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/article/{id}//related",
					Methods: []string{"GET"},
					Backend: hArticleShowRelated,
				},

				{
					Path:    "/article/slug/{month}/-/{day}/{year}",
					Methods: []string{"GET"},
					Backend: hArticleSlug,
				},

				{
					Path:    "/admin/user",
					Methods: []string{"GET"},
					Backend: hUserList,
				},

				{
					Path:    "/admin/user/",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/admin/user/",
					Methods: []string{"GET"},
					Backend: hUserList,
				},

				{
					Path:    "/admin/user//{id}",
					Methods: []string{"GET"},
					Backend: hUserShow,
				},

				{
					Path:    "/admin/user/{id}",
					Methods: []string{"GET"},
					Backend: hUserShow,
				},

				{
					Path:    "/admin/apps/{id}",
					Methods: []string{"GET"},
					Backend: hAdminAppShow,
				},

				{
					Path:    "/admin/apps/{id}/*",
					Methods: []string{"GET"},
					Backend: hAdminAppShowCatchall,
				},

				{
					Path:    "/admin/*",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/admin/*",
					Methods: []string{"GET"},
					Backend: hAdminCatchall,
				},

				{
					Path:    "/users/{userID}/profile",
					Methods: []string{"GET"},
					Backend: hUserProfile,
				},

				{
					Path:    "/users/super/*",
					Methods: []string{"GET"},
					Backend: hUserSuper,
				},

				{
					Path:    "/users/*",
					Methods: []string{"GET"},
					Backend: hUserAll,
				},

				{
					Path:    "/hubs/{hubID}/view",
					Methods: []string{"GET"},
					Backend: hHubView1,
				},

				{
					Path:    "/hubs/{hubID}/view/*",
					Methods: []string{"GET"},
					Backend: hHubView2,
				},

				{
					Path:    "/hubs/{hubID}/*",
					Methods: []string{"GET"},
					Backend: "sr",
				},
				{
					Path:    "/hubs/{hubID}/users",
					Methods: []string{"GET"},
					Backend: hHubView3,
				},
			},
		},
	}

	rules.Init()

	tests := []struct {
		r string   // input request path
		h string   // output matched handler
		k []string // output param keys
		v []string // output param values
	}{
		{r: "/", h: hIndex, k: nil, v: nil},
		{r: "/favicon.ico", h: hFavicon, k: nil, v: nil},

		{r: "/pages", h: "", k: nil, v: nil},
		{r: "/pages/", h: hStub, k: []string{egWildcard}, v: []string{""}},
		{r: "/pages/yes", h: hStub, k: []string{egWildcard}, v: []string{"yes"}},

		{r: "/article", h: hArticleList, k: nil, v: nil},
		{r: "/article/", h: hArticleList, k: nil, v: nil},
		{r: "/article/near", h: hArticleNear, k: nil, v: nil},
		{r: "/article/neard", h: hStub, k: []string{"id"}, v: []string{"neard"}},
		{r: "/article/123", h: hStub, k: []string{"id"}, v: []string{"123"}},
		{r: "/article/123/456", h: hArticleShowOpts, k: []string{"sup", "opts"}, v: []string{"123", "456"}},
		{r: "/article/@peter", h: hArticleByUser, k: []string{"user"}, v: []string{"peter"}},
		{r: "/article/22//related", h: hArticleShowRelated, k: []string{"id"}, v: []string{"22"}},
		{r: "/article/111/edit", h: hStub, k: []string{"iffd"}, v: []string{"111"}},
		{r: "/article/slug/sept/-/4/2015", h: hArticleSlug, k: []string{"month", "day", "year"}, v: []string{"sept", "4", "2015"}},
		{r: "/article/:id", h: hStub, k: []string{"id"}, v: []string{":id"}},

		{r: "/admin/user", h: hUserList, k: nil, v: nil},
		{r: "/admin/user/", h: hStub, k: nil, v: nil},
		{r: "/admin/user/1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/user//1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/hi", h: hStub, k: []string{egWildcard}, v: []string{"hi"}},
		{r: "/admin/lots/of/:fun", h: hStub, k: []string{egWildcard}, v: []string{"lots/of/:fun"}},
		{r: "/admin/apps/333", h: hAdminAppShow, k: []string{"id"}, v: []string{"333"}},
		{r: "/admin/apps/333/woot", h: hAdminAppShowCatchall, k: []string{"id", egWildcard}, v: []string{"333", "woot"}},

		{r: "/hubs/123/view", h: hHubView1, k: []string{"hubID"}, v: []string{"123"}},
		{r: "/hubs/123/view/index.html", h: hHubView2, k: []string{"hubID", egWildcard}, v: []string{"123", "index.html"}},
		{r: "/hubs/123/users", h: hHubView3, k: []string{"hubID"}, v: []string{"123"}},

		{r: "/users/123/profile", h: hUserProfile, k: []string{"userID"}, v: []string{"123"}},
		{r: "/users/super/123/okay/yes", h: hUserSuper, k: []string{egWildcard}, v: []string{"123/okay/yes"}},
		{r: "/users/123/okay/yes", h: hUserAll, k: []string{egWildcard}, v: []string{"123/okay/yes"}},
	}

	router := kind.CreateInstance(rules).(*radixTreeRouter)
	assert := assert.New(t)

	for _, tt := range tests {
		stdr, _ := http.NewRequest(http.MethodGet, tt.r, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)

		var backend string

		if context.Route != nil {
			backend = context.Route.GetBackend()
		}

		assert.Equal(tt.h, backend)

		paramKeys := context.Params.Keys
		paramValues := context.Params.Values

		assert.Equal(tt.k, paramKeys)
		assert.Equal(tt.v, paramValues)
	}
}

func TestTreeMoar(t *testing.T) {
	hStub := "hStub"
	hStub1 := "hStub1"
	hStub2 := "hStub2"
	hStub3 := "hStub3"
	hStub4 := "hStub4"
	hStub5 := "hStub5"
	hStub6 := "hStub6"
	hStub7 := "hStub7"
	hStub8 := "hStub8"
	hStub9 := "hStub9"
	hStub10 := "hStub10"
	hStub11 := "hStub11"
	hStub12 := "hStub12"
	hStub13 := "hStub13"
	hStub14 := "hStub14"
	hStub15 := "hStub15"
	hStub16 := "hStub16"

	// TODO: panic if we see {id}{x} because we're missing a delimiter, its not possible.
	// also {:id}* is not possible.

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/articlefun",
					Methods: []string{"GET"},
					Backend: hStub5,
				},

				{
					Path:    "/articles/{id}",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/articles/{slug}",
					Methods: []string{"DELETE"},
					Backend: hStub8,
				},

				{
					Path:    "/articles/search",
					Methods: []string{"GET"},
					Backend: hStub1,
				},

				{
					Path:    "/articles/{id}:delete",
					Methods: []string{"GET"},
					Backend: hStub8,
				},

				{
					Path:    "/articles/{iidd}!sup",
					Methods: []string{"GET"},
					Backend: hStub4,
				},

				{
					Path:    "/articles/{id}:{op}",
					Methods: []string{"GET"},
					Backend: hStub3,
				},

				{
					Path:    "/articles/{id}:{op}",
					Methods: []string{"GET"},
					Backend: hStub2,
				},

				{
					Path:    "/articles/{slug:^[a-z]+}/posts",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/articles/{id}/posts/{pid}",
					Methods: []string{"GET"},
					Backend: hStub6,
				},

				{
					Path:    "/articles/{id}/posts/{month}/{day}/{year}/{slug}",
					Methods: []string{"GET"},
					Backend: hStub7,
				},

				{
					Path:    "/articles/{id}.json",
					Methods: []string{"GET"},
					Backend: hStub10,
				},

				{
					Path:    "/articles/{id}/data.json",
					Methods: []string{"GET"},
					Backend: hStub11,
				},

				{
					Path:    "/articles/files/{file}.{ext}",
					Methods: []string{"GET"},
					Backend: hStub12,
				},

				{
					Path:    "/articles/me",
					Methods: []string{"PUT"},
					Backend: hStub13,
				},

				{
					Path:    "/pages/*",
					Methods: []string{"GET"},
					Backend: hStub,
				},

				{
					Path:    "/pages/*",
					Methods: []string{"GET"},
					Backend: hStub9,
				},

				{
					Path:    "/users/{id}",
					Methods: []string{"GET"},
					Backend: hStub14,
				},

				{
					Path:    "/users/{id}/settings/{key}",
					Methods: []string{"GET"},
					Backend: hStub15,
				},

				{
					Path:    "/users/{id}/settings/*",
					Methods: []string{"GET"},
					Backend: hStub16,
				},
			},
		},
	}

	tests := []struct {
		h string
		r string
		m string
		k []string
		v []string
	}{
		{m: "GET", r: "/articles/search", h: hStub1, k: nil, v: nil},
		{m: "GET", r: "/articlefun", h: hStub5, k: nil, v: nil},
		{m: "GET", r: "/articles/123", h: hStub, k: []string{"id"}, v: []string{"123"}},
		{m: "DELETE", r: "/articles/123mm", h: hStub8, k: []string{"slug"}, v: []string{"123mm"}},
		{m: "GET", r: "/articles/789:delete", h: hStub8, k: []string{"id"}, v: []string{"789"}},
		{m: "GET", r: "/articles/789!sup", h: hStub4, k: []string{"iidd"}, v: []string{"789"}},
		{m: "GET", r: "/articles/123:sync", h: hStub3, k: []string{"id", "op"}, v: []string{"123", "sync"}},
		{m: "GET", r: "/articles/456/posts/1", h: hStub6, k: []string{"id", "pid"}, v: []string{"456", "1"}},
		{m: "GET", r: "/articles/456/posts/09/04/1984/juice", h: hStub7, k: []string{"id", "month", "day", "year", "slug"}, v: []string{"456", "09", "04", "1984", "juice"}},
		{m: "GET", r: "/articles/456.json", h: hStub10, k: []string{"id"}, v: []string{"456"}},
		{m: "GET", r: "/articles/456/data.json", h: hStub11, k: []string{"id"}, v: []string{"456"}},
		{m: "GET", r: "/articles/files/file.zip", h: hStub12, k: []string{"file", "ext"}, v: []string{"file", "zip"}},
		{m: "GET", r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},
		{m: "GET", r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},
		{m: "PUT", r: "/articles/me", h: hStub13, k: nil, v: nil},
		{m: "GET", r: "/articles/me", h: hStub, k: []string{"id"}, v: []string{"me"}},
		{m: "GET", r: "/pages", h: "", k: nil, v: nil},
		{m: "GET", r: "/pages/", h: hStub, k: []string{egWildcard}, v: []string{""}},
		{m: "GET", r: "/pages/yes", h: hStub, k: []string{egWildcard}, v: []string{"yes"}},
		{m: "GET", r: "/users/1", h: hStub14, k: []string{"id"}, v: []string{"1"}},
		{m: "GET", r: "/users/", h: "", k: nil, v: nil},
		{m: "GET", r: "/users/2/settings/password", h: hStub15, k: []string{"id", "key"}, v: []string{"2", "password"}},
		{m: "GET", r: "/users/2/settings/", h: hStub16, k: []string{"id", egWildcard}, v: []string{"2", ""}},
	}

	rules.Init()

	router := kind.CreateInstance(rules).(*radixTreeRouter)
	assert := assert.New(t)

	for _, tt := range tests {
		stdr, _ := http.NewRequest(tt.m, tt.r, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)

		var backend string

		if context.Route != nil {
			backend = context.Route.GetBackend()
		}

		assert.Equal(tt.h, backend)

		paramKeys := context.Params.Keys
		paramValues := context.Params.Values

		assert.Equal(tt.k, paramKeys)
		assert.Equal(tt.v, paramValues)
	}
}

func TestTreeRegexp(t *testing.T) {
	hStub1 := "hStub1"
	hStub2 := "hStub2"
	hStub3 := "hStub3"
	hStub4 := "hStub4"
	hStub5 := "hStub5"
	hStub6 := "hStub6"
	hStub7 := "hStub7"

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/articles/{rid:^[0-9]{5,6}}",
					Methods: []string{"GET"},
					Backend: hStub7,
				},

				{
					Path:    "/articles/{zid:^0[0-9]+}",
					Methods: []string{"GET"},
					Backend: hStub3,
				},

				{
					Path:    "/articles/{name:^@[a-z]+}/posts",
					Methods: []string{"GET"},
					Backend: hStub4,
				},

				{
					Path:    "/articles/{op:^[0-9]+}/run",
					Methods: []string{"GET"},
					Backend: hStub5,
				},

				{
					Path:    "/articles/{id:^[0-9]+}",
					Methods: []string{"GET"},
					Backend: hStub1,
				},

				{
					Path:    "/articles/{id:^[1-9]+}-{aux}",
					Methods: []string{"GET"},
					Backend: hStub6,
				},

				{
					Path:    "/articles/{slug}",
					Methods: []string{"GET"},
					Backend: hStub2,
				},
			},
		},
	}

	tests := []struct {
		r string   // input request path
		h string   // output matched handler
		k []string // output param keys
		v []string // output param values
	}{
		{r: "/articles", h: "", k: nil, v: nil},
		{r: "/articles/12345", h: hStub7, k: []string{"rid"}, v: []string{"12345"}},
		{r: "/articles/123", h: hStub1, k: []string{"id"}, v: []string{"123"}},
		{r: "/articles/how-to-build-a-router", h: hStub2, k: []string{"slug"}, v: []string{"how-to-build-a-router"}},
		{r: "/articles/0456", h: hStub3, k: []string{"zid"}, v: []string{"0456"}},
		{r: "/articles/@pk/posts", h: hStub4, k: []string{"name"}, v: []string{"@pk"}},
		{r: "/articles/1/run", h: hStub5, k: []string{"op"}, v: []string{"1"}},
		{r: "/articles/1122", h: hStub1, k: []string{"id"}, v: []string{"1122"}},
		{r: "/articles/1122-yes", h: hStub6, k: []string{"id", "aux"}, v: []string{"1122", "yes"}},
	}
	rules.Init()
	router := kind.CreateInstance(rules).(*radixTreeRouter)
	assert := assert.New(t)

	for _, tt := range tests {
		stdr, _ := http.NewRequest(http.MethodGet, tt.r, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)

		var backend string

		if context.Route != nil {
			backend = context.Route.GetBackend()
		}

		assert.Equal(tt.h, backend)

		paramKeys := context.Params.Keys
		paramValues := context.Params.Values

		assert.Equal(tt.k, paramKeys)
		assert.Equal(tt.v, paramValues)
	}
}

func TestTreeRegexpRecursive(t *testing.T) {
	hStub1 := "hStub1"
	hStub2 := "hStub2"

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/one/{firstId:[a-z0-9-]+}/{secondId:[a-z0-9-]+}/first",
					Methods: []string{"GET"},
					Backend: hStub1,
				},

				{
					Path:    "/one/{firstId:[a-z0-9-_]+}/{secondId:[a-z0-9-_]+}/second",
					Methods: []string{"GET"},
					Backend: hStub2,
				},
			},
		},
	}

	tests := []struct {
		r string   // input request path
		h string   // output matched handler
		k []string // output param keys
		v []string // output param values
	}{
		{r: "/one/hello/world/first", h: hStub1, k: []string{"firstId", "secondId"}, v: []string{"hello", "world"}},
		{r: "/one/hi_there/ok/second", h: hStub2, k: []string{"firstId", "secondId"}, v: []string{"hi_there", "ok"}},
		{r: "/one///first", h: "", k: nil, v: nil},
		{r: "/one/hi/123/second", h: hStub2, k: []string{"firstId", "secondId"}, v: []string{"hi", "123"}},
	}
	rules.Init()
	router := kind.CreateInstance(rules).(*radixTreeRouter)
	assert := assert.New(t)

	for _, tt := range tests {
		stdr, _ := http.NewRequest(http.MethodGet, tt.r, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)

		var backend string

		if context.Route != nil {
			backend = context.Route.GetBackend()
		}

		assert.Equal(tt.h, backend)

		paramKeys := context.Params.Keys
		paramValues := context.Params.Values

		assert.Equal(tt.k, paramKeys)
		assert.Equal(tt.v, paramValues)
	}
}

func TestTreeRegexMatchWholeParam(t *testing.T) {
	hStub1 := "hStub1"
	hStub2 := "hStub1"
	hStub3 := "hStub1"

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/{id:[0-9]+}",
					Methods: []string{"GET"},
					Backend: hStub1,
				},

				{
					Path:    "/{x:.+}/foo",
					Methods: []string{"GET"},
					Backend: hStub2,
				},
				{
					Path:    "/{param:[0-9]*}/test",
					Methods: []string{"GET"},
					Backend: hStub3,
				},
			},
		},
	}

	tests := []struct {
		expectedHandler string
		url             string
	}{
		{url: "/13", expectedHandler: hStub1},
		{url: "/a13", expectedHandler: ""},
		{url: "/13.jpg", expectedHandler: ""},
		{url: "/a13.jpg", expectedHandler: ""},
		{url: "/a/foo", expectedHandler: hStub2},
		{url: "//foo", expectedHandler: ""},
		{url: "//test", expectedHandler: ""},
	}
	rules.Init()
	router := kind.CreateInstance(rules).(*radixTreeRouter)
	assert := assert.New(t)

	for _, tt := range tests {
		stdr, _ := http.NewRequest(http.MethodGet, tt.url, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)

		var backend string

		if context.Route != nil {
			backend = context.Route.GetBackend()
		}

		assert.Equal(tt.expectedHandler, backend)

	}
}

func BenchmarkTreeGet(b *testing.B) {
	h1 := "h1"
	h2 := "h2"

	rules := routers.Rules{
		{
			Paths: routers.Paths{
				{
					Path:    "/",
					Methods: []string{"GET"},
					Backend: h1,
				},

				{
					Path:    "/ping",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/pingall",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/ping/{id}",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/ping/{id}/woop",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/ping/{id}/{opt}",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/pinggggg",
					Methods: []string{"GET"},
					Backend: h2,
				},

				{
					Path:    "/hello",
					Methods: []string{"GET"},
					Backend: h1,
				},
			},
		},
	}
	rules.Init()
	router := kind.CreateInstance(rules).(*radixTreeRouter)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stdr, _ := http.NewRequest(http.MethodGet, "/ping/123/456", nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		router.Search(context)
	}
}

func TestRouteInitWrite(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		path    string
		rewrite string
		isPanic bool
		result  bool
	}{
		{
			path:    "/{name}/test/{demo}",
			rewrite: "",
			result:  false,
			isPanic: false,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{{ .na}me}}/popup/{desc}",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: `{{"put" | printf "%s%s" "out" | printf "%q"}}`,
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{desc}",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{demo}/{desc}",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/name/test/demo",
			rewrite: "/api/activity/ex/{name}/popup/{demo}/{desc}",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{demo}/*",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{demo}/{*}",
			result:  false,
			isPanic: true,
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{demo}",
			result:  true,
			isPanic: false,
		},

		{
			path:    "/{name}/test/{demo}/*",
			rewrite: "/api/activity/ex/{name}/popup/{demo}/{EG_WILDCARD}",
			result:  true,
			isPanic: false,
		},
	}

	for _, test := range tests {
		path := &routers.Path{
			Path:          test.path,
			RewriteTarget: test.rewrite,
		}
		var r *muxPath
		if test.isPanic {
			assert.Panics(func() { r = newMuxPath(path) })
		} else {
			assert.NotPanics(func() { r = newMuxPath(path) })
			assert.Equal(test.result, r.rewriteTemplate != nil)
		}

	}
}

func TestRouteRewrite(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		path         string
		rewrite      string
		result       string
		keys, values []string
	}{
		{
			path:    "/name/test/demo",
			rewrite: "",
			result:  "/name/test/demo",
		},

		{
			path:    "/name/test/demo",
			rewrite: "/abc",
			result:  "/abc",
		},

		{
			path:    "/{name}/test/{demo}",
			rewrite: "/api/activity/ex/{name}/popup/{demo}",
			keys:    []string{"name", "demo"},
			values:  []string{"v1", "v2"},
			result:  "/api/activity/ex/v1/popup/v2",
		},

		{
			path:    "/{name}/test/*",
			rewrite: "/api/activity/ex/{name}/popup/{EG_WILDCARD}",
			keys:    []string{"name", "EG_WILDCARD"},
			values:  []string{"v1", "v2"},
			result:  "/api/activity/ex/v1/popup/v2",
		},
	}

	for _, test := range tests {
		stdr, _ := http.NewRequest(http.MethodGet, test.path, nil)
		req, _ := httpprot.NewRequest(stdr)
		context := routers.NewContext(req)
		context.Params.Keys = test.keys
		context.Params.Values = test.values

		r := newMuxPath(&routers.Path{
			Path:          test.path,
			RewriteTarget: test.rewrite,
		})

		r.Rewrite(context)

		assert.Equal(test.result, req.Path())

	}
}
