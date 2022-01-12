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

package headerlookup

import (
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"

	cluster "github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createHeaderLookup(yamlSpec string, supervisor *supervisor.Supervisor) (*HeaderLookup, error) {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)
	spec, err := httppipeline.NewFilterSpec(rawSpec, supervisor)
	if err != nil {
		return nil, err
	}
	hl := &HeaderLookup{}
	hl.Init(spec)
	hl.Inherit(spec, hl)
	return hl, nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func TestValidate(t *testing.T) {
	etcdDirName, err := ioutil.TempDir("", "etcd-headerlookup-test")
	check(err)
	defer os.RemoveAll(etcdDirName)
	clusterInstance := cluster.CreateClusterForTest(etcdDirName)
	var mockMap sync.Map
	supervisor := supervisor.NewMock(
		nil, clusterInstance, mockMap, mockMap, nil, nil, false, nil, nil)

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
	}

	for _, unvalidYaml := range unvalidYamls {
		if _, err := createHeaderLookup(unvalidYaml, supervisor); err == nil {
			t.Errorf("validate should return error")
		}
	}

	if _, err := createHeaderLookup(validYaml, supervisor); err != nil {
		t.Errorf("validate should not return error: %s", err.Error())
	}
}

func TestHandle(t *testing.T) {
	etcdDirName, err := ioutil.TempDir("", "etcd-headerlookup-test")
	check(err)
	defer os.RemoveAll(etcdDirName)
	clusterInstance := cluster.CreateClusterForTest(etcdDirName)
	var mockMap sync.Map
	supervisor := supervisor.NewMock(
		nil, clusterInstance, mockMap, mockMap, nil, nil, false, nil, nil)

	// let's put data to 'foobar'
	clusterInstance.Put("/custom-data/credentials/foobar",
		`
ext-id: 123456789
extra-entry: "extra"
`)

	const config = `
name: headerLookup
kind: HeaderLookup
headerKey: "X-AUTH-USER"
etcdPrefix: "credentials/"
headerSetters:
  - etcdKey: "ext-id"
    headerKey: "user-ext-id"
`
	hl, err := createHeaderLookup(config, supervisor)
	check(err)

	// 'foobar' is the id
	ctx := &contexttest.MockedHTTPContext{}
	header := httpheader.New(http.Header{})
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return header
	}

	hl.Handle(ctx) // does nothing as header missing

	if header.Get("user-ext-id") != "" {
		t.Errorf("header should not be set")
	}

	header.Set("X-AUTH-USER", "unknown-user")

	hl.Handle(ctx) // does nothing as user is missing

	if header.Get("user-ext-id") != "" {
		t.Errorf("header should be set")
	}

	header.Set("X-AUTH-USER", "foobar")

	hl.Handle(ctx) // now updates header

	if header.Get("user-ext-id") != "123456789" {
		t.Errorf("header should be set")
	}
}
