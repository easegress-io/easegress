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

package proxy

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestSpecValidate(t *testing.T) {
	assert := assert.New(t)

	spec := &RequestMatcherSpec{}
	assert.Error(spec.Validate())

	spec.Policy = "headerHash"
	spec.Permil = 100
	assert.Error(spec.Validate())

	spec.Headers = map[string]*StringMatcher{}
	spec.Headers["X-Test"] = &StringMatcher{
		Empty: true,
		Exact: "abc",
	}
	assert.Error(spec.Validate())

	spec.Headers["X-Test"] = &StringMatcher{Exact: "abc"}
	spec.URLs = append(spec.URLs, &MethodAndURLMatcher{
		URL: &StringMatcher{
			Empty: true,
			Exact: "abc",
		},
	})
	assert.Error(spec.Validate())

	spec.URLs[0] = &MethodAndURLMatcher{
		URL: &StringMatcher{Empty: true},
	}
	assert.Error(spec.Validate())

	spec.HeaderHashKey = "X-Test"
	assert.NoError(spec.Validate())
}

func TestRandomMatcher(t *testing.T) {
	rm := NewRequestMatcher(&RequestMatcherSpec{
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
	rm := NewRequestMatcher(&RequestMatcherSpec{
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
	rm := NewRequestMatcher(&RequestMatcherSpec{
		Policy: "unknownPolicy",
		Permil: 100,
	})
	switch rm.(type) {
	case *ipHashMatcher:
		break
	default:
		t.Error("should create an ip hash matcher")
	}

	rm = NewRequestMatcher(&RequestMatcherSpec{
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

func TestGeneralMatche(t *testing.T) {
	assert := assert.New(t)

	// match all headers
	rm := NewRequestMatcher(&RequestMatcherSpec{
		MatchAllHeaders: true,
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
	})

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	stdr.Header.Set("X-Test1", "test1")
	assert.False(rm.Match(req))

	stdr.Header.Set("X-Test2", "not-test2")
	assert.False(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		MatchAllHeaders: true,
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Empty: true, Exact: "test2"},
		},
	})

	stdr.Header.Del("X-Test2")
	assert.True(rm.Match(req))

	// match one header
	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Empty: true, Exact: "test2"},
		},
	})
	assert.True(rm.Match(req))

	stdr.Header.Del("X-Test1")
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
	})
	assert.False(rm.Match(req))

	// match urls
	stdr.Header.Set("X-Test1", "test1")

	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
		URLs: []*MethodAndURLMatcher{
			{
				Methods: []string{http.MethodGet},
				URL: &StringMatcher{
					Exact: "/abc",
				},
			},
		},
	})
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
		URLs: []*MethodAndURLMatcher{
			{
				Methods: []string{http.MethodGet},
				URL: &StringMatcher{
					Exact: "/abcd",
				},
			},
		},
	})
	assert.False(rm.Match(req))
}

func TestMethodAndURLMatcher(t *testing.T) {
	assert := assert.New(t)

	m := &MethodAndURLMatcher{
		URL: &StringMatcher{
			Exact: "/abc",
		},
	}
	assert.NoError(m.Validate())

	m.init()

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	assert.True(m.Match(req))

	m.Methods = append(m.Methods, http.MethodGet)
	assert.True(m.Match(req))

	m.Methods = []string{http.MethodPost}
	assert.False(m.Match(req))
}

func TestStringMatcher(t *testing.T) {
	assert := assert.New(t)

	// validation
	sm := &StringMatcher{Empty: true}
	assert.NoError(sm.Validate())
	sm.init()

	sm = &StringMatcher{Empty: true, Exact: "abc"}
	assert.Error(sm.Validate())

	sm = &StringMatcher{}
	assert.Error(sm.Validate())

	sm = &StringMatcher{RegEx: "^abc[0-9]+$"}
	assert.NoError(sm.Validate())
	sm.init()

	sm.Prefix = "/xyz"
	assert.NoError(sm.Validate())

	sm.Exact = "/abc"
	assert.NoError(sm.Validate())

	// match
	sm = &StringMatcher{Empty: true}
	assert.True(sm.Match(""))
	assert.False(sm.Match("abc"))

	sm = &StringMatcher{RegEx: "^abc[0-9]+$"}
	sm.init()
	assert.True(sm.Match("abc123"))
	assert.False(sm.Match("abc123d"))

	sm.Prefix = "/xyz"
	assert.True(sm.Match("/xyz123"))
	assert.False(sm.Match("/Xyz123"))

	sm.Exact = "/hello"
	assert.True(sm.Match("/hello"))
	assert.False(sm.Match("/Hello"))
}
