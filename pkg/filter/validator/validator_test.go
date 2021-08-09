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
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createValidator(yamlSpec string, prev *Validator) *Validator {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)
	spec, _ := httppipeline.NewFilterSpec(rawSpec, nil)
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

	v := createValidator(yamlSpec, nil)

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
	v := createValidator(yamlSpec, nil)

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

	v = createValidator(yamlSpec, v)
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
	v := createValidator(yamlSpec, nil)

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
	v := createValidator(yamlSpec, nil)
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
	v = createValidator(yamlSpec, nil)

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
	v := createValidator(yamlSpec, nil)

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
