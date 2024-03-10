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

package httpproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func newTestProxy(yamlConfig string, assert *assert.Assertions) *Proxy {
	rawSpec := make(map[string]interface{})
	err := codectool.Unmarshal([]byte(yamlConfig), &rawSpec)
	assert.NoError(err)

	spec, err := filters.NewSpec(nil, "", rawSpec)
	assert.NoError(err)

	proxy := kind.CreateInstance(spec).(*Proxy)

	proxy.super = supervisor.NewMock(option.New(), nil, nil,
		nil, false, nil, nil)

	proxy.Init()

	assert.Equal(kind, proxy.Kind())
	assert.Equal(spec, proxy.Spec())
	return proxy
}

func getCtx(stdr *http.Request) *context.Context {
	req, _ := httpprot.NewRequest(stdr)
	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)
	return ctx
}

func TestProxy(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
  - url: http://127.0.0.1:9095
  - url: http://127.0.0.1:9096
  - url: http://127.0.0.1:9097
  loadBalance:
    policy: roundRobin
- filter:
    headers:
      "X-Test":
        exact: testheader
  servers:
  - url: http://127.0.0.2:9095
  - url: http://127.0.0.2:9096
  - url: http://127.0.0.2:9097
  - url: http://127.0.0.2:9098
  loadBalance:
    policy: roundRobin
  timeout: 10ms
- filter:
    headers:
      "X-Test":
        exact: stream
  servers:
  - url: http://127.0.0.2:9095
  - url: http://127.0.0.2:9096
  - url: http://127.0.0.2:9097
  - url: http://127.0.0.2:9098
  loadBalance:
    policy: roundRobin
  timeout: 10ms
  serverMaxBodySize: -1
mirrorPool:
  filter:
    headers:
      "X-Mirror":
        exact: mirror
  servers:
  - url: http://127.0.0.3:9095
  - url: http://127.0.0.3:9096
  loadBalance:
    policy: roundRobin
compression:
  minLength: 1024
`
	proxy := newTestProxy(yamlConfig, assert)
	proxy.InjectResiliencePolicy(make(map[string]resilience.Policy))

	assert.Equal(2, len(proxy.candidatePools))
	assert.Equal(2, len(proxy.mirrorPool.spec.Servers))

	assert.NotNil(proxy.Status())

	fnSendRequest0 := func(r *http.Request, client *http.Client) (*http.Response, error) {
		return &http.Response{
			Header: http.Header{},
			Body:   io.NopCloser(strings.NewReader("this is the body")),
		}, nil
	}

	fnSendRequest1 := func(r *http.Request, client *http.Client) (*http.Response, error) {
		return nil, fmt.Errorf("mocked error")
	}

	fnSendRequest2 := func(r *http.Request, client *http.Client) (*http.Response, error) {
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(100 * time.Millisecond):
			break
		}
		return &http.Response{
			Header: http.Header{},
			Body:   io.NopCloser(strings.NewReader("this is the body")),
		}, nil
	}

	fnSendRequest3 := func(r *http.Request, client *http.Client) (*http.Response, error) {
		rw := httptest.NewRecorder()
		rw.WriteString("this is a body longer than 20 bytes")
		return rw.Result(), nil
	}

	// direct set fnSendRequest to different function will cause data race since we use goroutine
	// for mirror.
	var fnKind int32
	fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
		kind := atomic.LoadInt32(&fnKind)
		switch kind {
		case 0:
			return fnSendRequest0(r, client)
		case 1:
			return fnSendRequest1(r, client)
		case 2:
			return fnSendRequest2(r, client)
		case 3:
			return fnSendRequest3(r, client)
		}
		return nil, fmt.Errorf("unknown kind")
	}

	atomic.StoreInt32(&fnKind, 0)
	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Mirror", "mirror")
		ctx := getCtx(stdr)
		assert.Equal("", proxy.Handle(ctx))
	}

	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		ctx := getCtx(stdr)
		assert.Equal("", proxy.Handle(ctx))
	}

	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Test", "testheader")
		ctx := getCtx(stdr)
		assert.Equal("", proxy.Handle(ctx))
	}

	atomic.StoreInt32(&fnKind, 1)
	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Test", "testheader")
		ctx := getCtx(stdr)
		assert.NotEqual("", proxy.Handle(ctx))
	}

	atomic.StoreInt32(&fnKind, 2)
	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Test", "testheader")
		ctx := getCtx(stdr)
		assert.NotEqual("", proxy.Handle(ctx))
	}

	atomic.StoreInt32(&fnKind, 3)
	{
		stdr, _ := http.NewRequest(http.MethodGet, "https://www.megaease.com", nil)
		stdr.Header.Set("X-Test", "stream")
		ctx := getCtx(stdr)
		resp, _ := httpprot.NewResponse(nil)
		resp.HTTPHeader().Set("X-Foo", "foo")
		ctx.SetResponse(context.DefaultNamespace, resp)
		assert.Equal("", proxy.Handle(ctx))
		ctx.Finish()
		assert.NotEmpty(ctx.Tags())
	}

	proxy.Close()
}

func TestSpecValidate(t *testing.T) {
	assert := assert.New(t)

	// no main pool
	yamlConfig := `
name: proxy
kind: Proxy
pools:
- filter:
    headers:
      "X-Test":
        exact: testheader
  servers:
  - url: http://127.0.0.1:9095
`
	spec := &Spec{}
	err := codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// no servers and service discovery
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- spanName: test
`
	spec = &Spec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// two main pools
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
  - url: http://127.0.0.1:9095
- servers:
  - url: http://127.0.0.2:9096
`
	spec = &Spec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// mirror pool: no filter
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
  - url: http://127.0.0.1:9095
mirrorPool:
  servers:
  - url: http://127.0.0.3:9095
`
	spec = &Spec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// mirror pool: has cache
	yamlConfig = `
name: proxy
kind: Proxy
pools:
- servers:
  - url: http://127.0.0.1:9095
mirrorPool:
  filter:
    headers:
      "X-Mirror":
        exact: mirror
  servers:
  - url: http://127.0.0.3:9095
  memoryCache:
    Methods: [Get]
`
	spec = &Spec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())

	// health check for service discovery
	yamlConfig = `
name: proxy
kind: Proxy
pools:
  - serviceName: service
    loadBalance:
      healthCheck:
        interval: 3s
`
	spec = &Spec{}
	err = codectool.Unmarshal([]byte(yamlConfig), spec)
	assert.NoError(err)
	assert.Error(spec.Validate())
}

func TestTLSConfig(t *testing.T) {
	assert := assert.New(t)

	yamlConfig := `
name: proxy
kind: Proxy
mtls:
  certBase64: YWJjZGVmZw==
  keyBase64: YWJjZGVmZ2FkZg==
  rootCertBase64: YWJjM2VmZ2FkZg==
pools:
- servers:
  - url: http://127.0.0.1:9095
`

	proxy := newTestProxy(yamlConfig, assert)
	_, err := proxy.tlsConfig()
	assert.Error(err)

	yamlConfig = `
name: proxy
kind: Proxy
mtls:
  keyBase64: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ1M3YVJXYXNOTi9GVWgKRXBZcHFVRmhHL2k1SGY2dFl4REZ0djhvcUxBRGNjaThabWlZSFczWHJ3aVFENXdia2prZFpMMkRrZFdUbVFnUwpkVUYxS2NzREhIc2UyNDFlbGw0U3BVSXIrVndMZXplUnMwSm1PQ3FYWWxzTHJCUElyRzh6MXZzY0lnZ2tNVVBaCnl1aUJsc0FUWDJLenk1Rm9oZjJRcUthQmllZFA2cmtZU1YrV2Jqc0drcTlVODEyT0UrTFhONUtKaGhEQnF6TS8KNUlnSlgycnY0TXRjU0JwelRNS0h1YmcvdG5hVlM3Ykc1Wmpyc0ZDcWc5bTNJbUc0Ti9BbFVtMTRrU1M0UC9CRgpXMGFBcC9BYjNxT0hVbzlzaXRyK040aTJOckhjQWkyc3BFR29nTGgzS1JTN2ZvVldKSFkrY2hTTGRyMGZmRGxJCng3dXhvQWVWQWdNQkFBRUNnZ0VBQW9FVVpQaXEzWUJvZndqUEVHUzNIWTJaZnFZNU9nRlBQdDl3bCtQUUpDN2oKU2ZyQTI1N2N5V2xOVHc5RkROOUFJL1VjbWNwNWhtdDhUTHc4NGw5VSszZVh6WjNXV2Y5Y0dSdEI5bmZvanJXSgo2K3pQTytqSEtROWZGK0xWNzN5bzVJeE1lVjFISUQ3S3RrS1VGZWxZMnJ1c2RmNEpPMnZWTjRyNFU0cmpLMlNCCktLTkp1U0hxZ01LV21rVmZsS1RhSGlyY2ZBV09sc1ZxWjlzUnpKdTJpcUc1NmtzVFBnRG9LK05obnNyaEc5cEwKT0pwQzRCeDVaQ2J5eUdTYXpjWXNDOTRUQnV6a1E5cGJvcjJTMkl2dC9pc00zaFdhUFhKVk1uNVVpUEhQYWlpNgpvVFpDcFhWTlJidkJyOFVuZFo1ODZQRmJJUWVsN2wzODRGZ2I4Zk95b1FLQmdRRERFY1JsZUIvc0FBWTZjbzAxCkRLVEdXaG9JMFZKRUYwZDhBWlRkR0s1ZEc0R1RnTGhOWFpEZDA2blFITDhxenlwMnppWlNjQTVVbXRsTUFKaWcKenhXNVFDaHZyNGdzT0ZNKzVVWk5sNjU1SU9mL0VQVG1wbUI4ZjhGaDNFT1paRkpaa3h1eVY0eXVoZWpkZnZEYQozeVVzYnFEOU5WZURReWpRZFNzT3U0WHF2UUtCZ1FEQTBtUlpsNkNyZGlRWUdMaXk3ZmY5VUtpQkNZMjZ0c08wCk02am1YMy81V21KTkt0eXZSZFFsbkJtbExMVGEyTlN2WVNVbk9uYklXZGd0SmRWVzhMUEp5ejBKcy9mM3BEam0KUzJoQzZNdjJZb3VhemtQcWlmelh4ZGl4NTJSMXdNT1dPN1VONUJvbE5SVkRDSkNYdVQ1c0YwTDYxRG1YWFdBdwpKRW5mbU9mSnVRS0JnQ3ZxT2c2bDVublkzNDRVNzlrN2lYVG1IK3BRUlhieXpyTUtJQnRPVFNMRTZIenVnNDlYCk94L1ZZT3RyTFZaVDRUbHgyNHEvazFwVXFnckVMNWcwUnEyMzFlS2UzOGNrdndqdjBNM3pFZUpQR0N1Q0E4QlIKUUhPR3gyQmltQTFXV251ejlJNUh5M0lXejMvZDdoYzRHVVJSZTRqRmszZ0hqSTZ4Y2dvVkNXYjVBb0dBRFNrTwo4bEo0QTl2ZllNbW5LWWMyYXRLcmZZc2lZa0VCSUhaNks2Y08rL3pnUXJZUE0rTkhOSDN2L2ljTC9QZlpwRkswCkQzWmREeFdhdkpJZGVuNlpOc2VwVmRVenNuSkI4KzNub3RGeXdsRTlpQVpWK2xjS3E4dDBHOGhZUWZVekpEalYKQmFxdzRpTTZYVVhqWUllakxBdDJaZHBBU0FWMmdES3AzQm42ai9rQ2dZRUFubEtSLzNRUFROTEtkUTE3cWhHVApYYjBUUGtRdWlQQkU0aWJmTDN1NTZDaVNzd3h1RlM4S2szTkYxMTVrUVh4VWR2T3ZrU0lBOFlwU1dzWGdxZzh0CnRTQjBDcGlhTzNLOWxOWEh3ZmNzcHlYVENWaFg5NElvOWhEenZ1bGtqRmIzaG41ams5SFovM3FIWCs3Z1ErbXYKeU5iS3pZcnVlRUhzWWpMVzJmZlpSeXc9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K
  certBase64: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURJVENDQWdtZ0F3SUJBZ0lRU1FIYU5pMUlzd2hKcnZwcDBrRytRREFOQmdrcWhraUc5dzBCQVFzRkFEQVMKTVJBd0RnWURWUVFLRXdkQlkyMWxJRU52TUI0WERURTJNREV3TVRFMU1EUXdOVm9YRFRJMU1USXlPVEUxTURRdwpOVm93RWpFUU1BNEdBMVVFQ2hNSFFXTnRaU0JEYnpDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDCkFRb0NnZ0VCQUpMdHBGWnF3MDM4VlNFU2xpbXBRV0ViK0xrZC9xMWpFTVcyL3lpb3NBTnh5THhtYUpnZGJkZXYKQ0pBUG5CdVNPUjFrdllPUjFaT1pDQkoxUVhVcHl3TWNleDdialY2V1hoS2xRaXY1WEF0N041R3pRbVk0S3BkaQpXd3VzRThpc2J6UFcreHdpQ0NReFE5bks2SUdXd0JOZllyUExrV2lGL1pDb3BvR0o1MC9xdVJoSlg1WnVPd2FTCnIxVHpYWTRUNHRjM2tvbUdFTUdyTXova2lBbGZhdS9neTF4SUduTk13b2U1dUQrMmRwVkx0c2JsbU91d1VLcUQKMmJjaVliZzM4Q1ZTYlhpUkpMZy84RVZiUm9DbjhCdmVvNGRTajJ5SzJ2NDNpTFkyc2R3Q0xheWtRYWlBdUhjcApGTHQraFZZa2RqNXlGSXQydlI5OE9Vakh1N0dnQjVVQ0F3RUFBYU56TUhFd0RnWURWUjBQQVFIL0JBUURBZ0trCk1CTUdBMVVkSlFRTU1Bb0dDQ3NHQVFVRkJ3TUJNQThHQTFVZEV3RUIvd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUUKRkdNb0xXMW5tWkU0LzhodVhyTkJjRzdHK0txcU1Cb0dBMVVkRVFRVE1CR0NDV3h2WTJGc2FHOXpkSWNFZndBQQpBVEFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBaGlOK1VUQjlpMUd5QzZnUTRCeDZ4VlJIUzJnYjFoVUlKU3pJClhXT1h5cEMxVjlNUnpHeWJsQTdQYzhJUHllWlRGQkkyUS9xWUNMaWh2S3hRbzZuUk5zQU5zdVFqRGtNakpVUkYKQldhQzZwQzNWRVZ5YURtNnYzelVYcWczZllSbDJvalc0dkZQbDhrdkkxbGxWaGxZZEs0VjVVVTI5R1h0WklJZgpPQTlJa0JVZDJwMVBmQ0J0QWRrc21qTVBWczZUalBzVHdHd2dKa0FqVUlwV1ZjRTUzT1JSQ1JsRDZLK2xDc1RLClBqVGRteXpMeEUyQTltT2xhMEVac3JaNGh5ZmVlMW9rU1dQMFFRUmNVQU1MNDlPR2JMOXY5RXZPaFVac29lczYKWUgzcUVjYWVhaFlJSG00ZnZ3aUJkRUpyT0RaUmt2V2l2ZDlVUU9Lc25BSzI0TENWcEE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  rootCertBase64: YWJjM2VmZ2FkZg==
pools:
- servers:
  - url: http://127.0.0.1:9095
`
	proxy = newTestProxy(yamlConfig, assert)
	_, err = proxy.tlsConfig()
	assert.NoError(err)
}

func TestToMetrics(t *testing.T) {
	assert := assert.New(t)

	s := Status{
		MainPool: &ServerPoolStatus{
			Stat: &httpstat.Status{
				RequestMetric: httpstat.RequestMetric{},
			},
		},
		CandidatePools: []*ServerPoolStatus{
			{
				Stat: &httpstat.Status{
					RequestMetric: httpstat.RequestMetric{},
				},
			},
		},
		MirrorPool: &ServerPoolStatus{
			Stat: &httpstat.Status{
				RequestMetric: httpstat.RequestMetric{},
			},
		},
	}

	metrics := s.ToMetrics("test")
	assert.Equal(3, len(metrics))
}
