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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

// https://openid.net/specs/openid-connect-core-1_0.html
const (
	kindName       = "OIDCAdaptor"
	resultFiltered = "oidcFiltered"
)

var httpCli = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var kind = &filters.Kind{
	Name:        kindName,
	Description: "OIDCAdaptor implement OpenID Connect authorization code flow spec",
	Results:     []string{resultFiltered},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &OIDCAdaptor{spec: spec.(*Spec)}
	},
}

type store interface {
	put(key, value string, timeout time.Duration) error
	get(key string) string
}

type OIDCAdaptor struct {
	spec *Spec
	store

	// Following are user custom defined
	jwksRefreshInterval  string
	setAccessTokenHeader bool
	setIDTokenHeader     bool
	setUserInfoHeader    bool

	redirectPath string
	oidcConfig   *oidcConfig
	jwks         *keyfunc.JWKS
}

type Spec struct {
	filters.BaseSpec `yaml:",inline"`

	CookieName string `json:"cookieName"`

	ClientId     string `json:"clientId" jsonschema:"required"`
	ClientSecret string `json:"clientSecret" jsonschema:"required"`

	Discovery string `json:"discovery"`

	// If Discovery not configured, following should be configured for OAuth2
	AuthorizationEndpoint string `json:"authorizationEndpoint"`
	TokenEndpoint         string `json:"tokenEndpoint"`
	UserInfoEndpoint      string `json:"userinfoEndpoint"`

	RedirectURI string `json:"redirectURI" jsonschema:"required"`
}

type oidcConfig struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	DeviceAuthorizationEndpoint       string   `json:"device_authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserInfoEndpoint                  string   `json:"userinfo_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	JwksUri                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IdTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
}

type oidcIDToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IdToken      string `json:"id_token"`
}

func init() {
	filters.Register(kind)
}

func (o *OIDCAdaptor) Name() string {
	return o.spec.Name()
}

func (o *OIDCAdaptor) Spec() filters.Spec {
	return o.spec
}

func (o *OIDCAdaptor) Kind() *filters.Kind {
	return kind
}

func (o *OIDCAdaptor) Init() {
	// delegate store interface operation to itself for testing
	o.store = o
	if len(o.spec.Discovery) > 0 {
		o.initDiscoveryOIDCConf()
	} else {
		o.oidcConfig = &oidcConfig{
			AuthorizationEndpoint: o.spec.AuthorizationEndpoint,
			TokenEndpoint:         o.spec.TokenEndpoint,
			UserInfoEndpoint:      o.spec.UserInfoEndpoint,
		}
	}
	o.setAccessTokenHeader = true
	o.setIDTokenHeader = true
	o.setUserInfoHeader = true
	parsed, err := url.Parse(o.spec.RedirectURI)
	if err != nil {
		logger.Errorf("parse redirectURI error: %s", err)
	}
	o.redirectPath = parsed.Path
}

func (o *OIDCAdaptor) Inherit(previousGeneration filters.Filter) {
	o.Init()
	previousGeneration.Close()
}

func (o *OIDCAdaptor) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	var rw *httpprot.Response
	if rw, _ = ctx.GetOutputResponse().(*httpprot.Response); rw == nil {
		rw, _ = httpprot.NewResponse(nil)
		ctx.SetOutputResponse(rw)
	}
	spec := o.spec

	if len(spec.CookieName) != 0 {
		if _, e := req.Cookie(spec.CookieName); e == nil {
			return ""
		}
	} else {
		var prefix = "Bearer"
		authHeader := req.HTTPHeader().Get("Authorization")
		if strings.HasPrefix(authHeader, prefix) && authHeader[len(prefix):] != "" {
			return ""
		}
	}
	if req.Path() == o.redirectPath {
		return o.handleOIDCCallback(ctx)
	}
	authorizeUrl := o.buildAuthorizeURL(req)
	rw.SetStatusCode(http.StatusFound)
	rw.Header().Set("Location", authorizeUrl)
	return resultFiltered
}

func (o *OIDCAdaptor) Status() interface{} {
	return nil
}

func (o *OIDCAdaptor) Close() {
}

func (o *OIDCAdaptor) initDiscoveryOIDCConf() {
	// https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfigurationRequest
	req, _ := http.NewRequest(http.MethodGet, o.spec.Discovery, nil)
	resp, err := httpCli.Do(req)
	var oidcConf oidcConfig
	err = readResp(resp, err, &oidcConf)
	if err != nil {
		logger.Errorf("req discovery endpoint['%s'] error: %s", err)
	}
	o.oidcConfig = &oidcConf
	var interval time.Duration
	if len(o.jwksRefreshInterval) == 0 {
		interval, err = time.ParseDuration(o.jwksRefreshInterval)
		if err != nil {
			logger.Errorf("parse jwksRefreshInterval[%s] duration error: %s", o.jwksRefreshInterval, err)
		}
	}
	jwks, err := keyfunc.Get(oidcConf.JwksUri, keyfunc.Options{
		Client:            httpCli,
		JWKUseNoWhitelist: true,
		RefreshInterval:   interval,
		ResponseExtractor: keyfunc.ResponseExtractorStatusAny,
	})
	if err != nil {
		logger.Errorf("failed to get the JWKS from the given URL error: %s", err)
	}
	o.jwks = jwks
}

func (o *OIDCAdaptor) handleOIDCCallback(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	rw := ctx.GetOutputResponse().(*httpprot.Response)
	spec := o.spec
	authCode := req.Std().URL.Query().Get("code")
	state := req.Std().URL.Query().Get("state")
	err := o.validateCodeAndState(authCode, state)
	if err != nil {
		return filterResp(rw, http.StatusForbidden, err.Error())
	}
	oidcToken, err := o.fetchOIDCToken(authCode, state, spec, err, rw, req)
	if err != nil {
		return errorResp(rw, "fetch OIDC token error: "+err.Error())
	}
	if o.setAccessTokenHeader {
		if len(req.HTTPHeader().Get("X-Access-Token")) == 0 {
			req.Header().Set("X-Access-Token", oidcToken.AccessToken)
		}
	}
	reqUrl := o.store.get(clusterCacheKey("request_url", state))
	req.Header().Set("X-Origin-Request-URL", reqUrl)

	userInfo := map[string]any{}
	if len(oidcToken.IdToken) > 0 {
		parseToken, err := o.validateIDToken(oidcToken.IdToken)
		if err != nil {
			return filterResp(rw, http.StatusUnauthorized, "invalid oidc id token")
		}
		if o.setIDTokenHeader {
			req.Header().Set("X-ID-Token", oidcToken.IdToken)
		}
		if claims, ok := parseToken.Claims.(jwt.MapClaims); ok {
			userInfo = claims
		}
	} else {
		err := o.fetchOAuth2Userinfo(authCode, oidcToken.AccessToken, &userInfo)
		if err != nil {
			return errorResp(rw, "fetch OAuth2 userinfo error: "+err.Error())
		}
	}
	if o.setUserInfoHeader {
		jsonBytes, err := json.Marshal(userInfo)
		if err != nil {
			logger.Errorf("marshal oidc userinfo to json error: %s", err)
		}
		req.Header().Set("X-User-Info", base64.StdEncoding.EncodeToString(jsonBytes))
	}
	return ""
}

func (o *OIDCAdaptor) fetchOIDCToken(authCode string, state string, spec *Spec, err error, rw *httpprot.Response, req *httpprot.Request) (*oidcIDToken, error) {
	// client_secret_post || client_secret_basic
	tokenFormData := url.Values{
		"client_id":     {o.spec.ClientId},
		"client_secret": {o.spec.ClientSecret},
		"code":          {authCode},
		"grant_type":    {"authorization_code"},
		"state":         {state},
		"redirect_uri":  {spec.RedirectURI},
	}
	// https://openid.net/specs/openid-connect-core-1_0.html#TokenRequest
	tokenReq, _ := http.NewRequest(http.MethodPost, o.oidcConfig.TokenEndpoint, strings.NewReader(tokenFormData.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authBasic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", spec.ClientId, spec.ClientSecret)))
	tokenReq.Header.Set("Authorization", "Basic "+authBasic)
	tokenReq.Header.Set("Accept", "application/json")

	resp, err := httpCli.Do(tokenReq)
	var oidcToken oidcIDToken
	err = readResp(resp, err, &oidcToken)
	if err != nil {
		logger.Errorf("handle oidc tokenRequest['%s'] error: %s", o.oidcConfig.TokenEndpoint, err)
		return nil, err
	}
	return &oidcToken, nil
}

func (o *OIDCAdaptor) fetchOAuth2Userinfo(authCode, accessToken string, userinfo *map[string]any) error {
	userinfoFormData := url.Values{
		"code":         {authCode},
		"grant_type":   {"authorization_code"},
		"redirect_uri": {o.spec.RedirectURI},
	}
	userinfoReq, _ := http.NewRequest(http.MethodGet, o.oidcConfig.UserInfoEndpoint, strings.NewReader(userinfoFormData.Encode()))
	userinfoReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	userinfoReq.Header.Set("Accept", "application/json")
	userinfoReq.Header.Set("Authorization", "token "+accessToken)
	userInfoResp, err := httpCli.Do(userinfoReq)
	return readResp(userInfoResp, err, userinfo)
}

func (o *OIDCAdaptor) validateIDToken(idJwtToken string) (*jwt.Token, error) {
	parseJwtToken, err := jwt.Parse(idJwtToken, func(token *jwt.Token) (interface{}, error) {
		alg := token.Method.Alg()
		// If the JWT alg Header Parameter uses a MAC based algorithm such as HS256, HS384, or HS512,
		// the octets of the UTF-8 representation of the client_secret corresponding to the client_id
		// contained in the aud (audience) Claim are used as the key to validate the signature
		if strings.HasPrefix(strings.ToLower(alg), "hs") {
			return []byte(o.spec.ClientSecret), nil
		}
		return o.jwks.Keyfunc(token)
	})
	if err != nil {
		return nil, err
	}
	if !parseJwtToken.Valid {
		return nil, fmt.Errorf("invalid id token")
	}
	return parseJwtToken, nil
}

func (o *OIDCAdaptor) validateCodeAndState(code, state string) error {
	errFmt := "oidc callback: %s"
	if len(state) == 0 {
		return fmt.Errorf(errFmt, "empty state")
	}
	cacheState := o.store.get(clusterCacheKey("state", state))
	if cacheState != "1" {
		return fmt.Errorf(errFmt, "invalid state")
	}
	if len(code) == 0 {
		return fmt.Errorf(errFmt, "invalid code")
	}
	return nil
}

func (o *OIDCAdaptor) buildAuthorizeURL(req *httpprot.Request) string {
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthorizationEndpoint
	var authUrlBuilder strings.Builder
	authUrlBuilder.WriteString(o.oidcConfig.AuthorizationEndpoint)
	authUrlBuilder.WriteString("?client_id=" + o.spec.ClientId)
	state := strings.ReplaceAll(uuid.New().String(), "-", "")
	// state is recommended
	authUrlBuilder.WriteString("&state=" + state)
	// End-user may spend some time doing login stuff, so we use a 10-minute timeout
	err := o.store.put(clusterCacheKey("state", state), "1", 10*time.Minute)
	if err != nil {
		logger.Errorf("put oidc state error: %s", err)
	}
	var reqUrl string
	args := req.URL().RawQuery
	if args != "" {
		reqUrl = fmt.Sprintf("%s://%s%s?%s", req.Scheme(), req.Host(), req.Path(), args)
	} else {
		reqUrl = fmt.Sprintf("%s://%s%s", req.Scheme(), req.Host(), req.Path())
	}
	err = o.store.put(clusterCacheKey("request_url", state), reqUrl, 10*time.Minute)
	if err != nil {
		logger.Errorf("put origin request url error: %s", err)
	}
	// nonce is optional
	nonce := strings.ReplaceAll(uuid.New().String(), "-", "")
	authUrlBuilder.WriteString("&nonce=" + nonce)
	authUrlBuilder.WriteString("&response_type=code")
	authUrlBuilder.WriteString("&scope=")
	if len(o.oidcConfig.ScopesSupported) > 0 {
		authUrlBuilder.WriteString(strings.Join(o.oidcConfig.ScopesSupported, "+"))
	} else {
		authUrlBuilder.WriteString("user")
	}
	authUrlBuilder.WriteString("&redirect_uri=" + url.QueryEscape(o.spec.RedirectURI))
	return authUrlBuilder.String()
}

func clusterCacheKey(tag string, val string) string {
	return "eg_oidc_key_" + tag + "_" + val
}

func readResp(resp *http.Response, err error, result any) error {
	if err != nil {
		return err
	}
	defer func(closer io.ReadCloser) {
		err := closer.Close()
		if err != nil {
			logger.Errorf("close error: %s", err)
		}
	}(resp.Body)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respBody, result)
	return err
}

func filterResp(resp *httpprot.Response, status int, payload any) string {
	resp.SetStatusCode(status)
	resp.SetPayload(payload)
	return resultFiltered
}

func errorResp(resp *httpprot.Response, payload any) string {
	resp.SetStatusCode(http.StatusInternalServerError)
	resp.SetPayload(payload)
	return resultFiltered
}

func (o *OIDCAdaptor) put(key, value string, timeout time.Duration) error {
	return o.spec.Super().Cluster().PutUnderTimeout(key, value, timeout)
}
func (o *OIDCAdaptor) get(key string) string {
	val, err := o.spec.Super().Cluster().Get(key)
	if err != nil {
		logger.Errorf("get value by key[%s] error: %s", key, err)
	}
	if val == nil {
		return ""
	}
	return *val
}
