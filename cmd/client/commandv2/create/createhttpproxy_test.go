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

// Package create provides create commands.
package create

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/megaease/easegress/v2/cmd/client/commandv2/specs"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func TestTranslateToProxyFilter(t *testing.T) {
	assert := assert.New(t)

	yamlStr := `
name: proxy
kind: Proxy
pools:
- servers:
  - url: http://127.0.0.1:9095
  - url: http://127.0.0.1:9096
  loadBalance:
    policy: roundRobin
`
	expected := specs.NewProxyFilterSpec("proxy")
	err := codectool.UnmarshalYAML([]byte(yamlStr), expected)
	assert.Nil(err)

	endpoints := []string{"http://127.0.0.1:9095", "http://127.0.0.1:9096"}
	got := translateToProxyFilter(endpoints)
	assert.Equal(expected, got)

	got2 := translateToProxyFilter(append(endpoints, "http://127.0.0.1:9097"))
	assert.NotEqual(expected, got2)
}

func TestTranslateToPipeline(t *testing.T) {
	assert := assert.New(t)

	yamlStr := `
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
    - url: http://127.0.0.1:9096
    loadBalance:
      policy: roundRobin
`
	// compare expected and got pipeline
	expected := specs.NewPipelineSpec("pipeline")
	err := codectool.UnmarshalYAML([]byte(yamlStr), expected)
	assert.Nil(err)

	endpoints := []string{"http://127.0.0.1:9095", "http://127.0.0.1:9096"}
	got := specs.NewPipelineSpec("pipeline")
	translateToPipeline(endpoints, got)

	// filters part is not compare here, because the filter part is map[string]interface{},
	// the expected map[string]interface{} is unmarshal from yaml,
	// the got map[string]interface{} is marshal from Proxy filter spec.
	// they have same valid information, but different content.
	assert.Equal(expected.Flow, got.Flow)
	assert.Equal(expected.Resilience, got.Resilience)
	assert.Equal(expected.Data, got.Data)
	assert.Len(got.Filters, 1)

	// compare expected and got filter
	// the expected filter is unmarshal twice from yaml,
	// if marshal it once, some part of expectedFilter will be nil.
	// but gotFilter will be empty. for example []string{} vs nil.
	// []string{} and nil are actually same in this case.
	expectedFilter := specs.NewProxyFilterSpec("proxy")
	filterYaml := codectool.MustMarshalYAML(expected.Filters[0])
	err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
	assert.Nil(err)
	filterYaml = codectool.MustMarshalYAML(expectedFilter)
	err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
	assert.Nil(err)

	gotFilter := specs.NewProxyFilterSpec("proxy")
	filterYaml = codectool.MustMarshalYAML(got.Filters[0])
	err = codectool.UnmarshalYAML(filterYaml, gotFilter)
	assert.Nil(err)

	assert.Equal(expectedFilter, gotFilter)
}

func TestParseRule(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		rule       string
		host       string
		path       string
		pathPrefix string
		endpoints  []string
		hasErr     bool
	}{
		{
			rule:       "foo.com/bar/bala=http://127.0.0.1:9096",
			host:       "foo.com",
			path:       "/bar/bala",
			pathPrefix: "",
			endpoints:  []string{"http://127.0.0.1:9096"},
			hasErr:     false,
		},
		{
			rule:       "/bar=http://127.0.0.1:9096,http://127.0.0.1:9097",
			host:       "",
			path:       "/bar",
			pathPrefix: "",
			endpoints:  []string{"http://127.0.0.1:9096", "http://127.0.0.1:9097"},
			hasErr:     false,
		},
		{
			rule:       "foo.com/bar*=http://127.0.0.1:9096",
			host:       "foo.com",
			path:       "",
			pathPrefix: "/bar",
			endpoints:  []string{"http://127.0.0.1:9096"},
			hasErr:     false,
		},
		{
			rule:       "/=http://127.0.0.1:9096",
			host:       "",
			path:       "/",
			pathPrefix: "",
			endpoints:  []string{"http://127.0.0.1:9096"},
			hasErr:     false,
		},
		{
			rule:   "foo.com/bar*=http://127.0.0.1:9096=",
			hasErr: true,
		},
		{
			rule:   "foo.com=http://127.0.0.1:9096",
			hasErr: true,
		},
		{
			rule:   "=http://127.0.0.1:9096",
			hasErr: true,
		},
		{
			rule:   "foo.com/path=",
			hasErr: true,
		},
		{
			rule:   "foo.com/path",
			hasErr: true,
		},
	}

	for _, tc := range testCases {
		rule, err := parseRule(tc.rule)
		if tc.hasErr {
			assert.NotNil(err, "case %v", tc)
			continue
		}
		assert.Nil(err, "case %v", tc)
		assert.Equal(tc.host, rule.Host, "case %v", tc)
		assert.Equal(tc.path, rule.Path, "case %v", tc)
		assert.Equal(tc.pathPrefix, rule.PathPrefix, "case %v", tc)
		assert.Equal(tc.endpoints, rule.Endpoints, "case %v", tc)
	}
}

func TestLoadCertFile(t *testing.T) {
	assert := assert.New(t)

	_, err := loadCertFile("not-exist-file.cert")
	assert.NotNil(err)

	text := "hello"
	textBase64 := "aGVsbG8="
	fileDir, err := os.MkdirTemp("", "test-load-cert-file")
	filePath := filepath.Join(fileDir, "test.cert")
	assert.Nil(err)
	defer os.RemoveAll(fileDir)
	os.WriteFile(filePath, []byte(text), 0644)

	got, err := loadCertFile(filePath)
	assert.Nil(err)
	assert.Equal(textBase64, got)
}

func TestToGeneralSpec(t *testing.T) {
	assert := assert.New(t)

	_, err := toGeneralSpec([]byte("not-yaml"))
	assert.NotNil(err)

	testStruct := struct {
		Name  string `yaml:"name"`
		Kind  string `yaml:"kind"`
		Other string `yaml:"other"`
	}{
		Name:  "test",
		Kind:  "test",
		Other: "other",
	}
	spec, err := toGeneralSpec(testStruct)
	assert.Nil(err)
	assert.Equal(testStruct.Name, spec.Name)
	assert.Equal(testStruct.Kind, spec.Kind)

	doc := `name: test
kind: test
other: other
`
	assert.Equal(doc, spec.Doc())
}

func TestCreateHTTPProxyOptions(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := os.MkdirTemp("", "test-create-http-proxy-options")
	assert.Nil(err)
	defer os.RemoveAll(tempDir)

	createCert := func(name string) string {
		p := filepath.Join(tempDir, name)
		os.WriteFile(p, []byte("hello"), 0644)
		return p
	}
	certBase64 := "aGVsbG8="

	o := &HTTPProxyOptions{
		Port: 10080,
		Rules: []string{
			"foo.com/barz=http://127.0.0.1:9095",
			"foo.com/bar*=http://127.0.0.1:9095",
			"/bar=http://127.0.0.1:9095",
		},
		TLS:                true,
		AutoCert:           true,
		CaCertFile:         createCert("ca.cert"),
		CertFiles:          []string{createCert("cert1"), createCert("cert2")},
		KeyFiles:           []string{createCert("key1"), createCert("key2")},
		AutoCertEmail:      "someone@easegress.com",
		AutoCertDomainName: "*.easegress.example",
		AutoCertDNSProvider: []string{
			"name=dnspod",
			"zone=easegress.com",
			"apiToken=abc",
		},
	}
	o.Complete([]string{"test"})
	err = o.Parse()
	assert.Nil(err)

	// auto cert
	assert.Equal("dnspod", o.dnsProvider["name"])
	assert.Equal("easegress.com", o.dnsProvider["zone"])
	assert.Equal("abc", o.dnsProvider["apiToken"])

	hs, pls := o.Translate()

	// meta
	assert.Equal("test", hs.Name)
	assert.Equal("HTTPServer", hs.Kind)
	assert.Equal(uint16(10080), hs.Port)

	// tls
	assert.True(hs.HTTPS)
	assert.True(hs.AutoCert)
	assert.Equal(certBase64, hs.CaCertBase64)
	assert.Equal(len(hs.Certs), len(hs.Keys))
	for k, v := range hs.Certs {
		assert.Equal(certBase64, v)
		assert.Equal(certBase64, hs.Keys[k])
	}

	// rules, host foo.com has two path, host "" has one path
	assert.Len(hs.Rules, 2)
	assert.Equal("foo.com", hs.Rules[0].Host)
	assert.Equal(routers.Paths{
		{Path: "/barz", Backend: "test-0"},
		{PathPrefix: "/bar", Backend: "test-1"},
	}, hs.Rules[0].Paths)
	assert.Equal(routers.Paths{
		{Path: "/bar", Backend: "test-2"},
	}, hs.Rules[1].Paths)

	// pipelines
	assert.Len(pls, 3)
	for i, pl := range pls {
		assert.Equal(fmt.Sprintf("test-%d", i), pl.Name)
		assert.Equal("Pipeline", pl.Kind)
		assert.Len(pl.Filters, 1)
	}

	yamlStr := `
filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095
    loadBalance:
      policy: roundRobin
`
	expectedFilter := func() *httpproxy.Spec {
		expected := specs.NewPipelineSpec("pipeline")
		err = codectool.UnmarshalYAML([]byte(yamlStr), expected)
		assert.Nil(err)

		expectedFilter := specs.NewProxyFilterSpec("proxy")
		filterYaml := codectool.MustMarshalYAML(expected.Filters[0])
		err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
		assert.Nil(err)
		filterYaml = codectool.MustMarshalYAML(expectedFilter)
		err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
		assert.Nil(err)
		return expectedFilter
	}()

	for i, p := range pls {
		gotFilter := specs.NewProxyFilterSpec("proxy")
		filterYaml := codectool.MustMarshalYAML(p.Filters[0])
		err = codectool.UnmarshalYAML(filterYaml, gotFilter)
		assert.Nil(err)
		assert.Equal(expectedFilter, gotFilter, i)
	}
}

func TestCreateHTTPProxyCmd(t *testing.T) {
	cmd := HTTPProxyCmd()
	assert.NotNil(t, cmd)

	resetOption := func() {
		httpProxyOptions = &HTTPProxyOptions{
			Port: 10080,
			Rules: []string{
				"foo.com/bar=http://127.0.0.1:9096",
			},
		}
	}
	resetOption()
	err := httpProxyArgs(cmd, []string{"demo"})
	assert.Nil(t, err)

	// test arg len
	err = httpProxyArgs(cmd, []string{})
	assert.NotNil(t, err)
	err = httpProxyArgs(cmd, []string{"demo", "123"})
	assert.NotNil(t, err)

	// test port
	httpProxyOptions.Port = -1
	err = httpProxyArgs(cmd, []string{"demo"})
	assert.NotNil(t, err)

	httpProxyOptions.Port = 65536
	err = httpProxyArgs(cmd, []string{"demo"})
	assert.NotNil(t, err)
	resetOption()

	// test rule
	httpProxyOptions.Rules = []string{}
	err = httpProxyArgs(cmd, []string{"demo"})
	assert.NotNil(t, err)
	resetOption()

	// test cert files
	httpProxyOptions.CertFiles = []string{"not-exist-file.cert"}
	err = httpProxyArgs(cmd, []string{"demo"})
	assert.NotNil(t, err)
	resetOption()

	// test run
	err = httpProxyRun(cmd, []string{"demo"})
	assert.NotNil(t, err)
}

func TestTranslateAutoCertManager(t *testing.T) {
	assert := assert.New(t)

	originalHook := handleReqHook
	defer func() {
		handleReqHook = originalHook
	}()
	handleReqHook = func(httpMethod string, path string, yamlBody []byte) ([]byte, error) {
		return []byte("[]"), nil
	}
	option := &HTTPProxyOptions{
		AutoCert:           true,
		AutoCertEmail:      "some@easegress.com",
		AutoCertDomainName: "*.easegress.example",
		AutoCertDNSProvider: []string{
			"name=dnspod",
			"zone=easegress.com",
			"apiToken=abc",
		},
	}
	option.Complete([]string{"test"})
	err := option.Parse()
	assert.Nil(err)

	spec, err := option.TranslateAutoCertManager()
	assert.Nil(err)
	assert.Equal("AutoCertManager", spec.Kind)
	assert.Equal("autocertmanager", spec.Name)
	assert.Equal("some@easegress.com", spec.Email)
	assert.Equal(1, len(spec.Domains))
	assert.Equal("*.easegress.example", spec.Domains[0].Name)
	assert.Equal(map[string]string{
		"name":     "dnspod",
		"zone":     "easegress.com",
		"apiToken": "abc",
	}, spec.Domains[0].DNSProvider)

	handleReqHook = func(httpMethod string, path string, yamlBody []byte) ([]byte, error) {
		return []byte(`[
			{
				"kind": "AutoCertManager",
				"name": "autocert",
				"email": "anybody@easegress.com",
				domains: [
					{
						"name": "*.easegress.org",
						"dnsProvider": {
							"name": "dnspod",
							"zone": "easegress.org",
							"apiToken": "abc"
						}
					}
				]
			}
		]`), nil
	}
	option = &HTTPProxyOptions{
		AutoCert:           true,
		AutoCertDomainName: "*.easegress.example",
		AutoCertDNSProvider: []string{
			"name=aliyun",
			"zone=easegress.com",
			"apiToken=abc",
		},
	}
	option.Complete([]string{"test"})
	err = option.Parse()
	assert.Nil(err)

	spec, err = option.TranslateAutoCertManager()
	assert.Nil(err)
	assert.Equal("AutoCertManager", spec.Kind)
	assert.Equal("autocert", spec.Name)
	assert.Equal("anybody@easegress.com", spec.Email)

	assert.Equal(2, len(spec.Domains))
	assert.Equal("*.easegress.org", spec.Domains[0].Name)
	assert.Equal("*.easegress.example", spec.Domains[1].Name)
}
