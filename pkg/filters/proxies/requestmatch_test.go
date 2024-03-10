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

package proxies

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/stretchr/testify/assert"
)

func TestRequestMatcherBaseSpecValidate(t *testing.T) {
	spec := &RequestMatcherBaseSpec{}
	assert.Error(t, spec.Validate())
	spec.Headers = map[string]*stringtool.StringMatcher{
		"test": {},
	}
	assert.Error(t, spec.Validate())

	spec.Headers = map[string]*stringtool.StringMatcher{
		"test": {Exact: "abc"},
	}
	assert.NoError(t, spec.Validate())

	spec.Policy = "headerHash"
	assert.Error(t, spec.Validate())

	spec.Permil = 100
	assert.Error(t, spec.Validate())

	spec.HeaderHashKey = "X-Test"
	assert.NoError(t, spec.Validate())
}

func TestRandomMatcher(t *testing.T) {
	rm := NewRequestMatcher(&RequestMatcherBaseSpec{
		Policy: "random",
		Permil: 100,
	})

	match := 0
	for i := 0; i < 10000; i++ {
		if rm.Match(nil) {
			match++
		}
	}

	if match < 900 || match > 1100 {
		t.Error("random matcher is not working as configured")
	}
}

func TestHeaderHashMatcher(t *testing.T) {
	rm := NewRequestMatcher(&RequestMatcherBaseSpec{
		Policy:        "headerHash",
		HeaderHashKey: "X-Test",
		Permil:        100,
	})

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	match := 0
	for i := 0; i < 10000; i++ {
		stdr.Header.Set("X-Test", strconv.Itoa(i))
		if rm.Match(req) {
			match++
		}
	}

	if match < 900 || match > 1100 {
		t.Error("header hash matcher is not working as configured")
	}
}

func TestIPHashMatcher(t *testing.T) {
	rm := NewRequestMatcher(&RequestMatcherBaseSpec{
		Policy: "unknownPolicy",
		Permil: 100,
	})
	switch rm.(type) {
	case *ipHashMatcher:
		break
	default:
		t.Error("should create an ip hash matcher")
	}

	rm = NewRequestMatcher(&RequestMatcherBaseSpec{
		Policy: "ipHash",
		Permil: 100,
	})

	stdr := &http.Request{Header: http.Header{}}

	match := 0
	for i := 0; i < 10000; i++ {
		a, b := i/256, i%256
		stdr.Header.Set("X-Real-Ip", fmt.Sprintf("192.168.%d.%d", a, b))
		req, _ := httpprot.NewRequest(stdr)
		if rm.Match(req) {
			match++
		}
	}

	if match < 900 || match > 1100 {
		t.Errorf("ip hash matcher is not working as configured")
	}
}
