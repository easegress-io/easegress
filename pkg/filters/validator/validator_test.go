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

	cluster "github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, id string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest(id, httpreq)
	ctx.UseRequest(id, id)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createValidator(yamlSpec string, prev *Validator, supervisor *supervisor.Supervisor) *Validator {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)
	spec, err := filters.NewSpec(supervisor, "", rawSpec)
	if err != nil {
		panic(err.Error())
	}
	v := &Validator{spec: spec.(*Spec)}
	if prev == nil {
		v.Init()
	} else {
		v.Inherit(prev)
	}
	return v
}

func TestHeaders(t *testing.T) {
	assert := assert.New(t)
	const yamlSpec = `
kind: Validator
name: validator
headers:
  Is-Valid:
    values: ["abc", "goodplan"]
    regexp: "^ok-.+$"
`

	v := createValidator(yamlSpec, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, "req", req)

	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("request has no header 'Is-Valid', should be invalid")
	}

	req.Header.Add("Is-Valid", "Invalid")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("request has header 'Is-Valid', but value is incorrect, should be invalid")
	}

	req.Header.Set("Is-Valid", "goodplan")
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("request has header 'Is-Valid' and value is correct, should be valid")
	}

	req.Header.Set("Is-Valid", "ok-1")
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("request has header 'Is-Valid' and matches the regular expression, should be valid")
	}
}

func TestJWT(t *testing.T) {
	assert := assert.New(t)
	const yamlSpec = `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: HS256
  secret: 313233343536
`
	v := createValidator(yamlSpec, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, "req", req)

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	req.Header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.keH6T3x1z7mmhKL1T3r9sQdAxxdzB6siemGMr_6ZOwU"
	req.Header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in header should be valid")
	}

	req.Header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	req.Header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	req.Header.Del("Authorization")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})
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
}

func TestOAuth2JWT(t *testing.T) {
	assert := assert.New(t)

	const yamlSpec = `
kind: Validator
name: validator
oauth2:
  jwt:
    algorithm: HS256
    secret: 313233343536
`
	v := createValidator(yamlSpec, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, "req", req)

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	req.Header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	req.Header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("OAuth/2 Authorization should succeed")
	}

	req.Header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	req.Header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func TestOAuth2TokenIntrospect(t *testing.T) {
	assert := assert.New(t)
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
	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, "req", req)

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	req.Header.Set("Authorization", "Bearer "+token)

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

	ctx := context.New(nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(t, err)
	setRequest(t, ctx, "req", req)

	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func prepareCtxAndHeader() (*context.Context, http.Header) {
	ctx := context.New(nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		panic(err)
	}
	setRequest(nil, ctx, "req", req)
	return ctx, req.Header
}

func cleanFile(userFile *os.File) {
	err := userFile.Truncate(0)
	check(err)
	_, err = userFile.Seek(0, 0)
	check(err)
	userFile.Write([]byte(""))
}

func TestBasicAuth(t *testing.T) {
	userIds := []string{
		"userY", "userZ", "nonExistingUser",
	}
	passwords := []string{
		"userpasswordY", "userpasswordZ", "userpasswordX",
	}
	encrypt := func(pw string) string {
		encPw, err := bcryptHash([]byte(pw))
		check(err)
		return encPw
	}
	encryptedPasswords := []string{
		encrypt("userpasswordY"), encrypt("userpasswordZ"), encrypt("userpasswordX"),
	}

	t.Run("unexisting userFile", func(t *testing.T) {
		yamlSpec := `
kind: Validator
name: validator
basicAuth:
  mode: FILE
  userFile: unexisting-file`
		v := createValidator(yamlSpec, nil, nil)
		ctx, _ := prepareCtxAndHeader()
		if v.Handle(ctx) != resultInvalid {
			t.Errorf("should be invalid")
		}
	})
	t.Run("credentials from userFile", func(t *testing.T) {
		userFile, err := os.CreateTemp("", "apache2-htpasswd")
		check(err)

		yamlSpec := `
kind: Validator
name: validator
basicAuth:
  mode: FILE
  userFile: ` + userFile.Name()

		// test invalid format
		userFile.Write([]byte("keypass"))
		v := createValidator(yamlSpec, nil, nil)
		ctx, _ := prepareCtxAndHeader()
		if v.Handle(ctx) != resultInvalid {
			t.Errorf("should be invalid")
		}

		// now proper format
		cleanFile(userFile)
		userFile.Write(
			[]byte(userIds[0] + ":" + encryptedPasswords[0] + "\n" + userIds[1] + ":" + encryptedPasswords[1]))
		expectedValid := []bool{true, true, false}

		v = createValidator(yamlSpec, nil, nil)

		t.Run("invalid headers", func(t *testing.T) {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0])) // missing : and pw
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if result != resultInvalid {
				t.Errorf("should be bad format")
			}
		})

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

		cleanFile(userFile) // no more authorized users

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

		os.Remove(userFile.Name())
		v.Close()
	})

	t.Run("test kvsToReader", func(t *testing.T) {
		kvs := make(map[string]string)
		kvs["/creds/key1"] = "key: key1\npass: pw"     // invalid
		kvs["/creds/key2"] = "ky: key2\npassword: pw"  // invalid
		kvs["/creds/key3"] = "key: key3\npassword: pw" // valid
		reader := kvsToReader(kvs)
		b, err := io.ReadAll(reader)
		check(err)
		s := string(b)
		if s != "key3:pw" {
			t.Errorf("parsing failed, %s", s)
		}
	})

	t.Run("dummy etcd", func(t *testing.T) {
		userCache := &etcdUserCache{prefix: ""} // should now skip cluster ops
		userCache.WatchChanges()
		if userCache.Match("doge", "dogepw") {
			t.Errorf("should not match")
		}
		userCache.Close()
	})

	t.Run("credentials from etcd", func(t *testing.T) {
		etcdDirName, err := ioutil.TempDir("", "etcd-validator-test")
		check(err)
		defer os.RemoveAll(etcdDirName)
		clusterInstance := cluster.CreateClusterForTest(etcdDirName)

		// Test newEtcdUserCache
		if euc := newEtcdUserCache(clusterInstance, ""); euc.prefix != "/custom-data/credentials/" {
			t.Errorf("newEtcdUserCache failed")
		}
		if euc := newEtcdUserCache(clusterInstance, "/extra-slash/"); euc.prefix != "/custom-data/extra-slash/" {
			t.Errorf("newEtcdUserCache failed")
		}

		pwToYaml := func(key string, user string, pw string) string {
			if user != "" {
				return fmt.Sprintf("username: %s\npassword: %s", user, pw)
			}
			return fmt.Sprintf("key: %s\npassword: %s", key, pw)
		}
		clusterInstance.Put("/custom-data/credentials/1", pwToYaml(userIds[0], "", encryptedPasswords[0]))
		clusterInstance.Put("/custom-data/credentials/2", pwToYaml("", userIds[2], encryptedPasswords[2]))

		var mockMap sync.Map
		supervisor := supervisor.NewMock(
			nil, clusterInstance, mockMap, mockMap, nil, nil, false, nil, nil)

		yamlSpec := `
kind: Validator
name: validator
basicAuth:
  mode: ETCD
  etcdPrefix: credentials/
`
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

		clusterInstance.Delete("/custom-data/credentials/1") // first user is not authorized anymore
		clusterInstance.Put("/custom-data/credentials/doge",
			`
randomEntry1: 21
nestedEntry:
  key1: val1
password: doge
key: doge
lastEntry: "byebye"
`)

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

		ctx, header := prepareCtxAndHeader()
		b64creds := base64.StdEncoding.EncodeToString([]byte("doge:doge"))
		header.Set("Authorization", "Basic "+b64creds)
		result := v.Handle(ctx)
		if result == resultInvalid {
			t.Errorf("should be authorized")
		}
		if header.Get("X-AUTH-USER") != "doge" {
			t.Errorf("x-auth-user header not set")
		}

		v.Close()
		wg := &sync.WaitGroup{}
		wg.Add(1)
		clusterInstance.CloseServer(wg)
		wg.Wait()
	})
}
