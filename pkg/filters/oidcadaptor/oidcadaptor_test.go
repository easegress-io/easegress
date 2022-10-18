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

package oidcadaptor

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

var googleOIDCConfig = `
name: authBackend
kind: Pipeline
flow:
  - filter: oidc-filter
    jumpIf: { oidcFiltered: END }
  - filter: proxy
filters:
  - name: oidc-filter
    kind: OIDCAdaptor
    cookieName: eg-oidc-cookie
    clientId: 
    clientSecret: 
    discovery: https://accounts.google.com/.well-known/openid-configuration
    redirectURI: /oidc/callback
  - name: proxy
    kind: Proxy
    pools:
      - loadBalance:
          policy: roundRobin
        servers:
          - url: http://127.0.0.1:8080
`

var githubOAuth2Config = `
name: authBackend
kind: Pipeline
flow:
  - filter: oidc-filter
    jumpIf: { oidcFiltered: END }
  - filter: proxy
filters:
  - name: oidc-filter
    kind: OIDCAdaptor
    cookieName: eg-oidc-cookie
    clientId: 
    clientSecret: 
    authorizationEndpoint: https://github.com/login/oauth/authorize
    tokenEndpoint: https://github.com/login/oauth/access_token
    userInfoEndpoint: https://api.github.com/user
    redirectURI: /oidc/callback
  - name: proxy
    kind: Proxy
    pools:
      - loadBalance:
          policy: roundRobin
        servers:
          - url: http://127.0.0.1:8080
`

func setRequest(t *testing.T, ctx *context.Context, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
}

type mockStore struct {
	s map[string]string
}

func (ms *mockStore) put(key, value string, timeout time.Duration) error {
	ms.s[key] = value
	return nil
}
func (ms *mockStore) get(key string) string {
	return ms.s[key]
}

func TestOidcRedirectUriParse(t *testing.T) {
	parsed, err := url.Parse("/oidc/callback")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "/oidc/callback", parsed.Path)
	parsed, err = url.Parse("https://example.com/oidc/callback")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "/oidc/callback", parsed.Path)
}
