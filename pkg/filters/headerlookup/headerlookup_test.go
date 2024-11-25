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

package headerlookup

import (
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"

	cluster "github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/cluster/clustertest"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createHeaderLookup(
	yamlConfig string, prev *HeaderLookup, supervisor *supervisor.Supervisor,
) (*HeaderLookup, error) {
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, err := filters.NewSpec(supervisor, "", rawSpec)
	if err != nil {
		return nil, err
	}
	hl := kind.CreateInstance(spec).(*HeaderLookup)
	if prev == nil {
		hl.Init()
	} else {
		hl.Inherit(prev)
	}
	return hl, nil
}

func createClusterAndSyncer() (*clustertest.MockedCluster, chan map[string]string) {
	clusterInstance := clustertest.NewMockedCluster()
	syncer := clustertest.NewMockedSyncer()
	clusterInstance.MockedSyncer = func(t time.Duration) (cluster.Syncer, error) {
		return syncer, nil
	}
	syncerChannel := make(chan map[string]string)
	syncer.MockedSyncPrefix = func(prefix string) (<-chan map[string]string, error) {
		return syncerChannel, nil
	}
	return clusterInstance, syncerChannel
}

func TestValidate(t *testing.T) {
	assert := assert.New(t)
	clusterInstance, _ := createClusterAndSyncer()

	supervisor := supervisor.NewMock(
		nil, clusterInstance, nil, nil, false, nil, nil)

	const validYaml = `
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "credentials/"
headerSetters:
  - etcdKey: "ext-id"
    headerKey: "user-ext-id"
`
	unvalidYamls := []string{
		`
name: headerLookup
kind: HeaderLookup
`,
		`
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
`,
		`
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "/credentials/"
`,
		`
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "/credentials/"
headerSetters:
  - etcdKey: "ext-id"
`,
		`
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "/credentials/"
headerSetters:
  - headerKey: "X-ext-id"
`,
		`
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
pathRegExp: "**"
etcdPrefix: "/credentials/"
headerSetters:
  - headerKey: "X-ext-id"
    etcdKey: "ext-id"
`,
	}

	for _, unvalidYaml := range unvalidYamls {
		_, err := createHeaderLookup(unvalidYaml, nil, supervisor)
		assert.NotNil(err)
	}

	_, err := createHeaderLookup(validYaml, nil, supervisor)
	assert.Nil(err)
}

func TestFindKeysToDelete(t *testing.T) {
	assert := assert.New(t)

	cache, _ := lru.New(10)
	kvs := make(map[string]string)
	kvs["doge"] = "headerA: 3\nheaderB: 6"
	kvs["foo"] = "headerA: 3\nheaderB: 232"
	kvs["bar"] = "headerA: 11\nheaderB: 43"
	kvs["key5"] = "headerA: 11\nheaderB: 43"
	kvs["key6"] = "headerA: 11\nheaderB: 43"
	cache.Add("doge", "headerA: 3\nheaderB: 6")   // same values
	cache.Add("foo", "headerA: 3\nheaderB: 232")  // new value
	cache.Add("key4", "---")                      // new value
	cache.Add("key6", "headerA: 11\nheaderB: 44") // new value
	res := findKeysToDelete(kvs, cache)
	sort.Strings(res)
	assert.NotEqual("foo", res[0])
	assert.NotEqual("key4", res[1])
}

func prepareCtxAndHeader(t *testing.T) (*context.Context, http.Header) {
	ctx := context.New(nil)
	stdReq, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	assert.Nil(t, err)
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
	header := stdReq.Header
	return ctx, header
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)

	const config = `
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "credentials/"
headerSetters:
  - etcdKey: "ext-id"
    headerKey: "user-ext-id"
`

	clusterInstance, syncerChannel := createClusterAndSyncer()

	supervisor := supervisor.NewMock(
		nil, clusterInstance, nil, nil, false, nil, nil)

	// let's put data to 'foobar'
	foobar := `
ext-id: "123456789"
extra-entry: "extra"
`
	clusterInstance.MockedGet = func(key string) (*string, error) {
		return &foobar, nil
	}
	hl, err := createHeaderLookup(config, nil, supervisor)
	assert.Equal(nil, err)

	// 'foobar' is the id
	ctx, header := prepareCtxAndHeader(t)

	hl.Handle(ctx) // does nothing as header missing
	assert.Equal("", header.Get("user-ext-id"))
	header.Set("X-AUTH-USER", "unknown-user")
	hl.Handle(ctx) // does nothing as user is missing
	assert.NotEqual("", header.Get("user-ext-id"))
	header.Set("X-AUTH-USER", "foobar")

	hl.Handle(ctx) // now updates header
	hdr1 := header.Get("user-ext-id")
	hl.Handle(ctx) // get from cache
	hdr2 := header.Get("user-ext-id")

	assert.Equal(hdr1, hdr2)
	assert.Equal("123456789", hdr1)

	hl, err = createHeaderLookup(config, hl, supervisor)
	assert.Nil(err)
	ctx, header = prepareCtxAndHeader(t)

	// update key-value store
	foobar = `
ext-id: "77341"
extra-entry: "extra"
`
	clusterInstance.MockedGet = func(key string) (*string, error) {
		return &foobar, nil
	}
	kvs := make(map[string]string)
	kvs["foobar"] = foobar
	syncerChannel <- kvs

	header.Set("X-AUTH-USER", "foobar")

	hl.Handle(ctx) // get updated value
	assert.Equal("77341", header.Get("user-ext-id"))

	hl, err = createHeaderLookup(config, hl, supervisor)
	assert.Nil(err)
	ctx, header = prepareCtxAndHeader(t)
	header.Set("X-AUTH-USER", "foobar")
	// delete foobar completely
	clusterInstance.MockedGet = func(key string) (*string, error) {
		return nil, nil
	}

	hl.Handle(ctx) // get updated value
	assert.Equal(0, len(header.Get("user-ext-id")))

	assert.Nil(hl.Status())
	assert.NotEqual(0, len(hl.Kind().Description))
	close(syncerChannel)
}

func TestHandleWithPath(t *testing.T) {
	assert := assert.New(t)

	const config = `
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "credentials/"
pathRegExp: "^/api/([a-z]+)/[0-9]*"
headerSetters:
  - etcdKey: "ext-id"
    headerKey: "user-ext-id"
`
	clusterInstance, _ := createClusterAndSyncer()
	supervisor := supervisor.NewMock(
		nil, clusterInstance, nil, nil, false, nil, nil)
	bobbanana := `
ext-id: "333"
extra-entry: "extra"
`
	bobpearl := `
ext-id: "4444"
extra-entry: "extra"
`

	clusterInstance.MockedGet = func(key string) (*string, error) {
		if key == "/custom-data/credentials/bob-bananas" {
			return &bobbanana, nil
		}
		if key == "/custom-data/credentials/bob-pearls" {
			return &bobpearl, nil
		}
		return nil, nil
	}

	hl, err := createHeaderLookup(config, nil, supervisor)
	assert.Nil(err)

	ctx, header := prepareCtxAndHeader(t)
	req := ctx.GetInputRequest().(*httpprot.Request)
	header.Set("X-AUTH-USER", "bob")
	hl.Handle(ctx) // path does not match
	assert.Equal("", header.Get("user-ext-id"))
	req.SetPath("/api/bananas/9281")
	hl.Handle(ctx)
	assert.Equal("333", header.Get("user-ext-id"))
	req.SetPath("/api/pearls/")
	hl.Handle(ctx)
	assert.Equal("4444", header.Get("user-ext-id"))

	hl.Close()
}
