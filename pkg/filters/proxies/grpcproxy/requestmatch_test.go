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

package grpcproxy

import (
	"context"
	"testing"

	"github.com/megaease/easegress/pkg/filters/proxies"
	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"google.golang.org/grpc/metadata"

	"github.com/stretchr/testify/assert"
)

func TestRequestMatcherSpecValidate(t *testing.T) {
	assert := assert.New(t)

	spec := &RequestMatcherSpec{}
	assert.Error(spec.Validate())

	spec.Policy = "headerHash"
	spec.Permil = 100
	assert.Error(spec.Validate())

	spec.Headers = map[string]*proxies.StringMatcher{}
	spec.Headers["X-Test"] = &proxies.StringMatcher{
		Empty: true,
		Exact: "abc",
	}
	assert.Error(spec.Validate())

	spec.Headers["X-Test"] = &proxies.StringMatcher{Exact: "abc"}
	spec.Methods = append(spec.Methods, &proxies.StringMatcher{
		Empty: true,
		Exact: "abc",
	})
	assert.Error(spec.Validate())

	spec.Methods[0] = &proxies.StringMatcher{Empty: true}
	assert.Error(spec.Validate())

	spec.HeaderHashKey = "X-Test"
	assert.NoError(spec.Validate())
}

func TestGeneralMatche(t *testing.T) {
	assert := assert.New(t)

	// match all headers
	rm := NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			MatchAllHeaders: true,
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
	})

	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(context.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)

	req.Header().Set("X-Test1", "test1")
	assert.False(rm.Match(req))

	req.Header().Set("X-Test2", "not-test2")
	assert.False(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			MatchAllHeaders: true,
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Empty: true, Exact: "test2"},
			},
		},
	})

	req.Header().Del("X-Test2")
	assert.True(rm.Match(req))

	// match one header
	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Empty: true, Exact: "test2"},
			},
		},
	})
	assert.True(rm.Match(req))

	req.Header().Del("X-Test1")
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
	})
	assert.False(rm.Match(req))

	// match urls
	req.Header().Set("X-Test1", "test1")
	req.SetFullMethod("/abc")
	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
		Methods: []*proxies.StringMatcher{
			{Exact: "/abc"},
		},
	})
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*proxies.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
		Methods: []*proxies.StringMatcher{
			{Exact: "/abcd"},
		},
	})
	assert.False(rm.Match(req))
}
