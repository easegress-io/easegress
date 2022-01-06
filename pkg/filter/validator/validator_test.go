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

package validator

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/phayes/freeport"

	cluster "github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/env"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createValidator(yamlSpec string, prev *Validator, supervisor *supervisor.Supervisor) *Validator {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)
	spec, err := httppipeline.NewFilterSpec(rawSpec, supervisor)
	if err != nil {
		panic(err.Error())
	}
	v := &Validator{}
	if prev == nil {
		v.Init(spec)
	} else {
		v.Inherit(spec, prev)
	}
	return v
}

func TestHeaders(t *testing.T) {
	const yamlSpec = `
kind: Validator
name: validator
headers:
  Is-Valid:
    values: ["abc", "goodplan"]
    regexp: "^ok-.+$"
`

	v := createValidator(yamlSpec, nil, nil)

	header := http.Header{}
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("request has no header 'Is-Valid', should be invalid")
	}

	header.Add("Is-Valid", "Invalid")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("request has header 'Is-Valid', but value is incorrect, should be invalid")
	}

	header.Set("Is-Valid", "goodplan")
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("request has header 'Is-Valid' and value is correct, should be valid")
	}

	header.Set("Is-Valid", "ok-1")
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("request has header 'Is-Valid' and matches the regular expression, should be valid")
	}
}

func TestJWT(t *testing.T) {
	const yamlSpec = `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: HS256
  secret: 313233343536
`
	v := createValidator(yamlSpec, nil, nil)

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedCookie = func(name string) (*http.Cookie, error) {
		return nil, fmt.Errorf("not exist")
	}
	header := http.Header{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.keH6T3x1z7mmhKL1T3r9sQdAxxdzB6siemGMr_6ZOwU"
	header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in header should be valid")
	}

	header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	header.Del("Authorization")
	ctx.MockedRequest.MockedCookie = func(name string) (*http.Cookie, error) {
		return &http.Cookie{Value: token}, nil
	}
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in cookie should be valid")
	}

	v = createValidator(yamlSpec, v, nil)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in cookie should be valid")
	}

	if v.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
	v.Description()
}

func TestOAuth2JWT(t *testing.T) {
	const yamlSpec = `
kind: Validator
name: validator
oauth2:
  jwt:
    algorithm: HS256
    secret: 313233343536
`
	v := createValidator(yamlSpec, nil, nil)

	ctx := &contexttest.MockedHTTPContext{}

	header := http.Header{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("OAuth/2 Authorization should succeed")
	}

	header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func TestOAuth2TokenIntrospect(t *testing.T) {
	yamlSpec := `
kind: Validator
name: validator
oauth2:
  tokenIntrospect:
    endPoint: http://oauth2.megaease.com/
    insecureTls: true
    clientId: megaease
    clientSecret: secret
`
	v := createValidator(yamlSpec, nil, nil)
	ctx := &contexttest.MockedHTTPContext{}

	header := http.Header{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	header.Set("Authorization", "Bearer "+token)

	body := `{
			"subject":"megaease.com",
			"scope":"read,write",
			"active": false
		}`
	fnSendRequest = func(client *http.Client, r *http.Request) (*http.Response, error) {
		reader := strings.NewReader(body)
		return &http.Response{
			Body: io.NopCloser(reader),
		}, nil
	}
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	yamlSpec = `
kind: Validator
name: validator
oauth2:
  tokenIntrospect:
    endPoint: http://oauth2.megaease.com/
    clientId: megaease
    clientSecret: secret
    basicAuth: megaease@megaease
`
	v = createValidator(yamlSpec, nil, nil)

	body = `{
			"subject":"megaease.com",
			"scope":"read,write",
			"active": true
		}`
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("OAuth/2 Authorization should succeed")
	}
}

func TestSignature(t *testing.T) {
	// This test is almost covered by signer

	const yamlSpec = `
kind: Validator
name: validator
signature:
  accessKeys:
    AKID: SECRET
`
	v := createValidator(yamlSpec, nil, nil)

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedStd = func() *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "http://megaease.com", nil)
		return r
	}

	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func TestBasicAuthUpdateCache(t *testing.T) {
	kvs1 := make(map[string]string)
	cache, _ := lru.New(16)
	for i := 0; i < 30; i++ {
		kvs1[fmt.Sprintf("u%d", i)] = fmt.Sprintf("md5-%d", i)
	}
	updateCache(kvs1, cache) // does nothing
	if cache.Len() != 0 {
		t.Errorf("cache should not be modified")
	}

	cache.Add("u0", "md5-0")     // present in kvs1
	cache.Add("u743", "md5-743") // not present in kvs1
	cache.Add("u744", "md5-744") // not present in kvs1

	if cache.Len() != 3 {
		t.Errorf("cache should have 3 items")
	}
	updateCache(kvs1, cache) // removes u743 and u744
	if cache.Len() != 1 {
		t.Errorf("cache should have 1 item")
	}

	cache.Add("u7", "md5-old-password") // present in kvs1 but different value
	updateCache(kvs1, cache)            // updates u7
	if val, ok := cache.Get("u7"); ok {
		if val.(string) != "md5-7" {
			t.Errorf("cache value should be updated")
		}
	} else {
		t.Errorf("cache should contain u7")
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func createCluster(tempDir string) cluster.Cluster {
	ports, err := freeport.GetFreePorts(3)
	check(err)
	name := fmt.Sprintf("test-member-x")
	opt := option.New()
	opt.Name = name
	opt.ClusterName = "test-cluster"
	opt.ClusterRole = "primary"
	opt.ClusterRequestTimeout = "10s"
	opt.Cluster.ListenClientURLs = []string{fmt.Sprintf("http://localhost:%d", ports[0])}
	opt.Cluster.AdvertiseClientURLs = opt.Cluster.ListenClientURLs
	opt.Cluster.ListenPeerURLs = []string{fmt.Sprintf("http://localhost:%d", ports[1])}
	opt.Cluster.InitialAdvertisePeerURLs = opt.Cluster.ListenPeerURLs
	opt.Cluster.InitialCluster = make(map[string]string)
	opt.Cluster.InitialCluster[name] = opt.Cluster.InitialAdvertisePeerURLs[0]
	opt.APIAddr = fmt.Sprintf("localhost:%d", ports[2])
	opt.DataDir = fmt.Sprintf("%s/data", tempDir)
	opt.LogDir = fmt.Sprintf("%s/log", tempDir)
	opt.MemberDir = fmt.Sprintf("%s/member", tempDir)

	_, err = opt.Parse()
	check(err)

	env.InitServerDir(opt)

	clusterInstance, err := cluster.New(opt)
	check(err)
	return clusterInstance
}

func prepareCtxAndHeader() (*contexttest.MockedHTTPContext, http.Header) {
	ctx := &contexttest.MockedHTTPContext{}
	header := http.Header{}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	return ctx, header
}

func TestBasicAuth(t *testing.T) {
	userIds := []string{
		"userY", "userZ", "nonExistingUser",
	}
	passwords := []string{
		"md5-encrypted-pw-1", "md5-encrypted-pw-2", "md5-encrypted-pw3",
	}
	t.Run("credentials from userFile", func(t *testing.T) {
		userFile, err := os.CreateTemp("", "apache2-htpasswd")
		check(err)

		yamlSpec := `
kind: Validator
name: validator
basicAuth:
  userFile: ` + userFile.Name()

		userFile.Write([]byte(userIds[0] + ":" + passwords[0] + "\n" + userIds[1] + ":" + passwords[1]))
		expectedValid := []bool{true, true, false}

		v := createValidator(yamlSpec, nil, nil)
		// set shorted syncInterval for test
		v.basicAuth.authorizedUsersCache = newHtpasswdUserCache(v.basicAuth.spec.UserFile, 200*time.Millisecond)
		go v.basicAuth.authorizedUsersCache.WatchChanges()

		for i := 0; i < 3; i++ {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[i] + ":" + passwords[i]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if expectedValid[i] {
				if result == resultInvalid {
					t.Errorf("should be authorized")
				}
			} else {
				if result != resultInvalid {
					t.Errorf("should be unauthorized")
				}
			}
		}

		v.basicAuth.authorizedUsersCache.Lock()
		err = userFile.Truncate(0)
		check(err)
		_, err = userFile.Seek(0, 0)
		check(err)
		userFile.Write([]byte("")) // no more authorized users
		v.basicAuth.authorizedUsersCache.Unlock()

		tryCount := 5
		for i := 0; i <= tryCount; i++ {
			time.Sleep(300 * time.Millisecond) // wait that cache item gets deleted
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0] + ":" + passwords[0]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if result == resultInvalid {
				break // successfully unauthorized
			}
			if i == tryCount && result != resultInvalid {
				t.Errorf("should be unauthorized")
			}
		}

		v.basicAuth.authorizedUsersCache.Lock()
		os.Remove(userFile.Name())
		v.basicAuth.authorizedUsersCache.Unlock()
		v.Close()
	})
	t.Run("credentials from etcd", func(t *testing.T) {
		etcdDirName, err := ioutil.TempDir("", "etcd-validator-test")
		check(err)
		defer os.RemoveAll(etcdDirName)
		clusterInstance := createCluster(etcdDirName)

		clusterInstance.Put("/credentials/"+userIds[0], passwords[0])
		clusterInstance.Put("/credentials/"+userIds[2], passwords[2])

		var mockMap sync.Map
		supervisor := supervisor.NewMock(
			nil, clusterInstance, mockMap, mockMap, nil, nil, false, nil, nil)

		yamlSpec := `
kind: Validator
name: validator
basicAuth:
  useEtcd: true`

		expectedValid := []bool{true, false, true}
		v := createValidator(yamlSpec, nil, supervisor)
		for i := 0; i < 3; i++ {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[i] + ":" + passwords[i]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if expectedValid[i] {
				if result == resultInvalid {
					t.Errorf("should be authorized")
				}
			} else {
				if result != resultInvalid {
					t.Errorf("should be unauthorized")
				}
			}
		}

		clusterInstance.Delete("/credentials/" + userIds[0]) // first user is not authorized anymore
		tryCount := 5
		for i := 0; i <= tryCount; i++ {
			time.Sleep(200 * time.Millisecond) // wait that cache item gets deleted
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0] + ":" + passwords[0]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if result == resultInvalid {
				break // successfully unauthorized
			}
			if i == tryCount && result != resultInvalid {
				t.Errorf("should be unauthorized")
			}
		}

		v.Close()
		wg := &sync.WaitGroup{}
		wg.Add(1)
		clusterInstance.CloseServer(wg)
		wg.Wait()
	})
}
