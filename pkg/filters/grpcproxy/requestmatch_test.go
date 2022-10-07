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

package grpcprxoy

import (
	"context"
	"fmt"
	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"google.golang.org/grpc/metadata"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestMatcherSpecValidate(t *testing.T) {
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
	spec.URLs = append(spec.URLs, &URLMatcher{
		URL: &StringMatcher{
			Empty: true,
			Exact: "abc",
		},
	})
	assert.Error(spec.Validate())

	spec.URLs[0] = &URLMatcher{
		URL: &StringMatcher{Empty: true},
	}
	assert.Error(spec.Validate())

	spec.HeaderHashKey = "X-Test"
	assert.NoError(spec.Validate())
}

func TestRandomMatcher(t *testing.T) {
	rand.Seed(0)

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

	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)

	match := 0
	for i := 0; i < 10000; i++ {
		req.Header().Set("X-Test", strconv.Itoa(i))
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

	match := 0
	for i := 0; i < 10000; i++ {
		a, b := i/256, i%256
		sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
		req := grpcprot.NewRequestWithServerStream(sm)
		req.SetRealIP(fmt.Sprintf("192.168.%d.%d", a, b))
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

	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)

	req.Header().Set("X-Test1", "test1")
	assert.False(rm.Match(req))

	req.Header().Set("X-Test2", "not-test2")
	assert.False(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		MatchAllHeaders: true,
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Empty: true, Exact: "test2"},
		},
	})

	req.Header().Del("X-Test2")
	assert.True(rm.Match(req))

	// match one header
	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Empty: true, Exact: "test2"},
		},
	})
	assert.True(rm.Match(req))

	req.Header().Del("X-Test1")
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
	})
	assert.False(rm.Match(req))

	// match urls
	req.Header().Set("X-Test1", "test1")
	req.SetFullMethod("/abc")
	rm = NewRequestMatcher(&RequestMatcherSpec{
		Headers: map[string]*StringMatcher{
			"X-Test1": {Exact: "test1"},
			"X-Test2": {Exact: "test2"},
		},
		URLs: []*URLMatcher{
			{
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
		URLs: []*URLMatcher{
			{
				URL: &StringMatcher{
					Exact: "/abcd",
				},
			},
		},
	})
	assert.False(rm.Match(req))
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
