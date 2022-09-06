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

package grpcserver

import (
	stdcontext "context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"testing"
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
	req.SetPath("/abc")

	_, ins := newTestMux(yamlSpec, assert)
	assert.Equal(codes.PermissionDenied, ins.search(req).code)

	req.SetRealIP("127.0.0.1")
	r := &route{code: 1}
	ins.putRouteToCache(req.Host(), req.Path(), r)
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
	req.SetPath("/abc")

	assert.Equal(codes.PermissionDenied, ins.search(req).code)

	req.SetRealIP("127.0.0.3")
	req.SetPath("/abc")
	assert.Equal(codes.NotFound, ins.search(req).code)
}

func TestSearchPath(t *testing.T) {
	assertions := assert.New(t)

	yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
rules:
- host: 127.0.0.1
  paths: 
  - path: "/abd"
    backend: "test-demo"
`
	_, ins := newTestMux(yamlSpec, assertions)
	md := metadata.MD{}
	md.Set(grpcprot.Authority, "127.0.0.1")
	sm := grpcprot.NewFakeServerStream(metadata.NewIncomingContext(stdcontext.Background(), md))
	request := grpcprot.NewRequestWithServerStream(sm)
	request.SetRealIP("127.0.0.1")
	request.SetHost("127.0.0.1")
	request.SetPath("/abc")
	search := ins.search(request)
	assertions.Equal(codes.NotFound, search.code)

	request.SetHost("127.0.0.2")
	request.SetPath("/abd")
	search = ins.search(request)
	assertions.Equal(codes.NotFound, search.code)

	request.SetHost("127.0.0.1")
	request.SetPath("/abd")
	search = ins.search(request)
	assertions.Equal(codes.OK, search.code)
	assertions.Equal("test-demo", search.path.backend)
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
 - paths:
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
	req.SetPath("/abc")
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

func TestPathRewrite(t *testing.T) {
	assertions := assert.New(t)

	t.Run("rewrite path by path", func(t *testing.T) {
		yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
rules:
 - paths:
   - path: /abc
     rewriteTarget: /abd
`
		superSpec, err := supervisor.NewSpec(yamlSpec)
		assertions.NoError(err)
		assertions.NotNil(superSpec)

		m := newMux(nil)
		m.reload(superSpec, &contexttest.MockedMuxMapper{})

		_, ins := newTestMux(yamlSpec, assertions)
		req := grpcprot.NewRequestWithServerStream(grpcprot.NewFakeServerStream(stdcontext.Background()))
		req.SetPath("/abc")
		req.SetRealIP("127.0.0.1")

		search := ins.search(req)
		search.path.rewrite(req)

		assertions.Equal("/abd", req.Path())
	})

	t.Run("rewrite path by path prefix", func(t *testing.T) {
		yamlSpec := `
kind: GRPCServer
maxConnections: 1024
maxConnectionIdle: 60s
port: 8850
name: server-grpc
rules:
 - paths:
   - pathPrefix: /abc
     rewriteTarget: /abde
`
		superSpec, err := supervisor.NewSpec(yamlSpec)
		assertions.NoError(err)
		assertions.NotNil(superSpec)

		m := newMux(nil)
		m.reload(superSpec, &contexttest.MockedMuxMapper{})

		_, ins := newTestMux(yamlSpec, assertions)
		req := grpcprot.NewRequestWithServerStream(grpcprot.NewFakeServerStream(stdcontext.Background()))
		req.SetPath("/abc/abc")
		req.SetRealIP("127.0.0.1")

		search := ins.search(req)
		search.path.rewrite(req)

		assertions.Equal("/abde/abc", req.Path())
	})

}
