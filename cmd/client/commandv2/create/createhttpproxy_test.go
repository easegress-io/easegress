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

// Package create provides create commands.
package create

import (
	"testing"

	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
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
	expected := filters.GetKind(httpproxy.Kind).DefaultSpec().(*httpproxy.Spec)
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
	expected := (&pipeline.Pipeline{}).DefaultSpec().(*pipeline.Spec)
	err := codectool.UnmarshalYAML([]byte(yamlStr), expected)
	assert.Nil(err)

	endpoints := []string{"http://127.0.0.1:9095", "http://127.0.0.1:9096"}
	got := translateToPipeline(endpoints)

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
	expectedFilter := filters.GetKind(httpproxy.Kind).DefaultSpec().(*httpproxy.Spec)
	filterYaml := codectool.MustMarshalYAML(expected.Filters[0])
	err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
	assert.Nil(err)
	filterYaml = codectool.MustMarshalYAML(expectedFilter)
	err = codectool.UnmarshalYAML(filterYaml, expectedFilter)
	assert.Nil(err)

	gotFilter := filters.GetKind(httpproxy.Kind).DefaultSpec().(*httpproxy.Spec)
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
