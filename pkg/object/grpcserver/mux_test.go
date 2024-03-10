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

package grpcserver

import (
	stdcontext "context"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

func newTestMux(yamlSpec string, at *assert.Assertions) (*mux, *muxInstance) {
	superSpec, err := supervisor.NewSpec(yamlSpec)
	at.NoError(err)
	at.NotNil(superSpec)

	m := newMux(nil)
	m.reload(superSpec, &contexttest.MockedMuxMapper{})
	ins, ok := m.inst.Load().(*muxInstance)
	at.True(ok)
	return m, ins
}

func TestReload(t *testing.T) {
	assert := assert.New(t)
	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
cacheSize: 1
`
	m, ins := newTestMux(yamlSpec, assert)
	superSpec, _ := supervisor.NewSpec(yamlSpec)
	m.reload(superSpec, &contexttest.MockedMuxMapper{})
	assert.True(ins != m.inst.Load())
}

func TestSearchCache(t *testing.T) {
	assert := assert.New(t)
	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
cacheSize: 1
`
	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(stdcontext.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)
	req.Header().Set(grpcprot.Authority, "127.0.0.1")
	req.SetFullMethod("/abc")

	_, ins := newTestMux(yamlSpec, assert)
	assert.Equal(codes.PermissionDenied, ins.search(req).code)

	req.SetRealIP("127.0.0.1")
	r := &route{code: 1}
	ins.putRouteToCache(req.Host(), req.FullMethod(), r)
	assert.True(r == ins.search(req))
}

func TestSearchIP(t *testing.T) {
	assert := assert.New(t)
	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
ipFilter: 
  blockByDefault: false
  blockIPs: ["127.0.0.2"]
`
	_, ins := newTestMux(yamlSpec, assert)
	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(stdcontext.Background(), metadata.MD{}))
	req := grpcprot.NewRequestWithServerStream(sm)
	req.SetRealIP("127.0.0.2")
	req.SetFullMethod("/abc")

	assert.Equal(codes.PermissionDenied, ins.search(req).code)

	req.SetRealIP("127.0.0.3")
	req.SetFullMethod("/abc")
	assert.Equal(codes.NotFound, ins.search(req).code)
}

func TestSearchMethod(t *testing.T) {
	assertions := assert.New(t)

	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
rules:
- host: 127.0.0.1
  methods: 
  - method: "/abd"
    backend: "test-demo"
`
	_, ins := newTestMux(yamlSpec, assertions)
	md := metadata.MD{}
	md.Set(grpcprot.Authority, "127.0.0.1")
	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(stdcontext.Background(), md))
	request := grpcprot.NewRequestWithServerStream(sm)
	request.SetRealIP("127.0.0.1")
	request.SetHost("127.0.0.1")
	request.SetFullMethod("/abc")
	search := ins.search(request)
	assertions.Equal(codes.NotFound, search.code)

	request.SetHost("127.0.0.2")
	request.SetFullMethod("/abd")
	search = ins.search(request)
	assertions.Equal(codes.NotFound, search.code)

	request.SetHost("127.0.0.1")
	request.SetFullMethod("/abd")
	search = ins.search(request)
	assertions.Equal(codes.OK, search.code)
	assertions.Equal("test-demo", search.method.backend)
}

func TestHeader(t *testing.T) {
	assertions := assert.New(t)

	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
rules:
 - methods:
   - backend: "test-demo"
     headers:
       - key: "array"
         values: ["1","2"]
       - key: "regex"
         regexp: "\\d"
`
	_, ins := newTestMux(yamlSpec, assertions)
	req := grpcprot.NewRequestWithServerStream(grpcprot.NewFakeServerStream(stdcontext.Background()))
	req.SetRealIP("127.0.0.1")
	req.SetFullMethod("/abc")
	req.Header().Set("array", "3")
	search := ins.search(req)
	assertions.Equal(codes.NotFound, search.code)

	req.Header().Add("array", "2")
	search = ins.search(req)
	assertions.Equal(codes.OK, search.code)

	req.Header().Del("array")
	req.Header().Set("regex", "w")
	search = ins.search(req)
	assertions.Equal(codes.NotFound, search.code)

	req.Header().Add("regex", "1")
	search = ins.search(req)
	assertions.Equal(codes.OK, search.code)
}

func TestNewMuxRule(t *testing.T) {
	rule := &Rule{}

	r := newMuxRule(nil, rule, nil)
	assert.Nil(t, r.hostRE)

	rule.HostRegexp = "("
	r = newMuxRule(nil, rule, nil)
	assert.Nil(t, r.hostRE)

	rule.HostRegexp = "/foo/abc[1-9]+"
	r = newMuxRule(nil, rule, nil)
	assert.NotNil(t, r.hostRE)
}

func TestMuxRuleMatch(t *testing.T) {
	rule := &Rule{}
	r := newMuxRule(nil, rule, nil)

	assert.True(t, r.match(""))
	assert.True(t, r.match("abc"))

	rule.Host = "abc"
	r = newMuxRule(nil, rule, nil)
	assert.False(t, r.match(""))
	assert.False(t, r.match("foo"))
	assert.True(t, r.match("abc"))

	rule.HostRegexp = "fo.+"
	r = newMuxRule(nil, rule, nil)
	assert.False(t, r.match(""))
	assert.True(t, r.match("foo"))
	assert.True(t, r.match("abc"))
	assert.False(t, r.match("abcd"))
}

func TestNewMuxMethod(t *testing.T) {
	method := &Method{}

	m := newMuxMethod(nil, method)
	assert.Nil(t, m.methodRE)

	method.MethodRegexp = "("
	m = newMuxMethod(nil, method)
	assert.Nil(t, m.methodRE)

	method.MethodRegexp = ".+T"
	m = newMuxMethod(nil, method)
	assert.NotNil(t, m.methodRE)
}

func TestMuxMethodMatchMethod(t *testing.T) {
	method := &Method{}

	m := newMuxMethod(nil, method)
	assert.True(t, m.matchMethod(""))
	assert.True(t, m.matchMethod("GET"))

	method.Method = "GET"
	m = newMuxMethod(nil, method)
	assert.False(t, m.matchMethod(""))
	assert.False(t, m.matchMethod("POST"))
	assert.True(t, m.matchMethod("GET"))

	method.MethodPrefix = "OP"
	m = newMuxMethod(nil, method)
	assert.False(t, m.matchMethod(""))
	assert.False(t, m.matchMethod("POST"))
	assert.True(t, m.matchMethod("OPTIONS"))

	method.MethodRegexp = "PO.+"
	m = newMuxMethod(nil, method)
	assert.False(t, m.matchMethod(""))
	assert.False(t, m.matchMethod("PUT"))
	assert.True(t, m.matchMethod("POST"))
}

func TestMuxMethodMatchHeaders(t *testing.T) {
	method := &Method{}

	stream := grpcprot.NewFakeServerStream(stdcontext.Background())
	req := grpcprot.NewRequestWithServerStream(stream)

	m := newMuxMethod(nil, method)
	assert.False(t, m.matchHeaders(req))

	method.MatchAllHeader = true
	m = newMuxMethod(nil, method)
	assert.True(t, m.matchHeaders(req))

	method.Headers = []*Header{
		{
			Key:    "foo",
			Values: []string{"foo1"},
		},
		{
			Key:    "bar",
			Values: []string{"", "bar1"},
		},
	}
	m = newMuxMethod(nil, method)
	assert.False(t, m.matchHeaders(req))

	req.Header().Add("foo", "foo1")
	assert.True(t, m.matchHeaders(req))

	m = newMuxMethod(nil, method)
	assert.True(t, m.matchHeaders(req))

	req.Header().Add("bar", "bar2")
	assert.False(t, m.matchHeaders(req))

	method.MatchAllHeader = false
	m = newMuxMethod(nil, method)
	req.Header().Del("foo")
	req.Header().Del("bar")
	assert.True(t, m.matchHeaders(req))

	req.Header().Add("bar", "bar1")
	assert.True(t, m.matchHeaders(req))

	req.Header().Set("bar", "bar2")
	assert.False(t, m.matchHeaders(req))
}

func TestAppendXForwardedFor(t *testing.T) {
	stream := grpcprot.NewFakeServerStream(stdcontext.Background())
	req := grpcprot.NewRequestWithServerStream(stream)
	req.SetRealIP("123.123.123.123")

	appendXForwardedFor(req)
	assert.Equal(t, "123.123.123.123", req.RawHeader().RawGet("X-Forwarded-For")[0])

	appendXForwardedFor(req)
	assert.Equal(t, 1, len(req.RawHeader().RawGet("X-Forwarded-For")))

	req.SetRealIP("1.1.1.1")
	appendXForwardedFor(req)
	assert.Equal(t, 2, len(req.RawHeader().RawGet("X-Forwarded-For")))
	assert.Equal(t, "123.123.123.123", req.RawHeader().RawGet("X-Forwarded-For")[0])
}

func TestMuxInstanceHandle(t *testing.T) {
	assertions := assert.New(t)

	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
xForwardedFor: true
rules:
- host: 127.0.0.1
  methods: 
  - method: "/abd"
    backend: "test-demo"
`
	_, inst := newTestMux(yamlSpec, assertions)

	stream := grpcprot.NewFakeServerStream(stdcontext.Background())
	req := grpcprot.NewRequestWithServerStream(stream)
	req.SetRealIP("1.1.1.1")

	req.SetFullMethod("/abd")
	result := inst.handler(req)
	assertions.NotEmpty(result)

	req.SetHost("127.0.0.1")
	result = inst.handler(req)
	assertions.NotEmpty(result)

	mmm := inst.muxMapper.(*contexttest.MockedMuxMapper)
	result = inst.handler(req)
	assertions.NotEmpty(result)

	mmm.MockedGetHandler = func(name string) (context.Handler, bool) {
		return &contexttest.MockedHandler{
			MockedHandle: func(ctx *context.Context) string {
				return ""
			},
		}, true
	}
	result = inst.handler(req)
	assertions.NotEmpty(result)

	mmm.MockedGetHandler = func(name string) (context.Handler, bool) {
		return &contexttest.MockedHandler{
			MockedHandle: func(ctx *context.Context) string {
				resp, _ := httpprot.NewResponse(nil)
				ctx.SetOutputResponse(resp)
				return ""
			},
		}, true
	}
	result = inst.handler(req)
	assertions.NotEmpty(result)

	mmm.MockedGetHandler = func(name string) (context.Handler, bool) {
		return &contexttest.MockedHandler{
			MockedHandle: func(ctx *context.Context) string {
				ctx.SetOutputResponse(grpcprot.NewResponse())
				return ""
			},
		}, true
	}

	result = inst.handler(req)
	assertions.Empty(result)

	mmm.MockedGetHandler = func(name string) (context.Handler, bool) {
		return &contexttest.MockedHandler{
			MockedHandle: func(ctx *context.Context) string {
				panic("test")
			},
		}, true
	}
	result = inst.handler(req)
	assertions.NotEmpty(result)
}
