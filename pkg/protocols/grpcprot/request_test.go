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

package grpcprot

import (
	"context"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestNewContext(t *testing.T) {
	assertions := assert.New(t)
	ctx := context.Background()
	fake := NewFakeServerStream(ctx)
	request := NewRequestWithServerStream(fake)
	assertions.NotEqual(ctx, request.Context())
}

func TestPath(t *testing.T) {
	assertions := assert.New(t)
	fake := NewFakeServerStream(context.Background())
	request := NewRequestWithServerStream(fake)
	assertions.Empty(request.FullMethod())
	request.SetFullMethod("/abc")
	assertions.Equal("/abc", request.FullMethod())
	method, ok := grpc.Method(request.Context())
	assertions.True(ok)
	assertions.Equal(request.FullMethod(), method)

}

func TestRealIP(t *testing.T) {
	assertions := assert.New(t)
	peerInfo := &peer.Peer{Addr: &Addr{addr: ""}}
	fake := NewFakeServerStream(peer.NewContext(context.Background(), peerInfo))
	assertions.Equal("", NewRequestWithServerStream(fake).RealIP())

	peerInfo = &peer.Peer{Addr: &Addr{addr: "127.0.0.1:8080"}}
	fake = NewFakeServerStream(peer.NewContext(context.Background(), peerInfo))
	req := NewRequestWithServerStream(fake)
	assertions.Equal("127.0.0.1", req.RealIP())

	req.SetRealIP("127.0.0.2")
	assertions.Equal("127.0.0.2", req.RealIP())
}

func TestSourceHost(t *testing.T) {
	assertions := assert.New(t)
	fake := NewFakeServerStream(peer.NewContext(context.Background(), &peer.Peer{Addr: &Addr{addr: "127.0.0.1:8080"}}))
	req := NewRequestWithServerStream(fake)
	assertions.Equal("127.0.0.1:8080", req.SourceHost())

	req.SetRealIP("0.0.0.0")
	assertions.Equal("0.0.0.0:8080", req.SourceHost())

}

func TestHost(t *testing.T) {
	assertions := assert.New(t)
	md := metadata.MD{}
	fake := NewFakeServerStream(metadata.NewIncomingContext(context.Background(), md))
	assertions.Equal("", NewRequestWithServerStream(fake).Host())

	md.Set(Authority, "127.0.0.1")
	assertions.Equal("127.0.0.1", NewRequestWithServerStream(fake).Host())
}

func TestHeadNotNilPanic(t *testing.T) {
	assertions := assert.New(t)
	fake := NewFakeServerStream(context.Background())
	assertions.NotPanics(func() {
		NewRequestWithServerStream(fake)
	})

	assertions.NotNil(NewRequestWithServerStream(fake).Header())
	assertions.NotNil(NewRequestWithServerStream(fake).RawHeader())
}

func TestHeaderPoint(t *testing.T) {
	assertions := assert.New(t)
	fake := NewFakeServerStream(context.Background())
	req := NewRequestWithServerStream(fake)
	assertions.Nil(req.Header().Get("test"))

	header := NewHeader(metadata.MD{})
	req.SetHeader(header)

	header.Set("test", "test")
	assertions.NotNil(req.Header().Get("test"))

	req.Header().Del("test")
	assertions.Equal(0, header.md.Len())

	header.Set("test", "test")
	assertions.NotNil(req.RawHeader())
	assertions.NotNil("test", req.RawHeader().GetFirst("test"))
}

// clone test cases create request from other request's context and check two request should be consistent
func TestClone(t *testing.T) {
	assertions := assert.New(t)
	src := NewRequestWithServerStream(NewFakeServerStream(context.Background()))
	src.SetHeader(NewHeader(metadata.New(nil)))
	src.SetFullMethod("/abc")
	src.SetRealIP("127.0.0.1")
	src.SetHost("127.0.0.2")
	src.Header().Set("test", "test")

	dst := NewRequestWithContext(src.Context())

	assertions.Equal(src.RealIP(), dst.RealIP())
	assertions.Equal(src.FullMethod(), dst.FullMethod())
	assertions.Equal(src.Host(), dst.Host())
	assertions.Equal(src.SourceHost(), src.SourceHost())
	assertions.NotNil(src.Header())
	assertions.NotNil(dst.Header())
	assertions.True(src.Header() != dst.Header())
	assertions.Equal(src.header.md.Len(), dst.header.md.Len())
	assertions.Equal(src.RawHeader().GetFirst("test"), dst.RawHeader().GetFirst("test"))

}

func TestAddr(t *testing.T) {
	assert := assert.New(t)

	var a *Addr
	assert.Equal("", a.Network())

	a = &Addr{addr: "123.123.123.123"}
	assert.Equal("tcp", a.Network())
	assert.Equal("123.123.123.123", a.String())

	a.setAddr("1.1.1.1")
	assert.Equal("1.1.1.1", a.String())
}

func TestGetServerStream(t *testing.T) {
	assert := assert.New(t)
	ss := NewFakeServerStream(context.Background())
	req := NewRequestWithServerStream(ss)
	assert.Equal(ss, req.GetServerStream())
}

func TestSimpleRequestFunctions(t *testing.T) {
	ss := NewFakeServerStream(context.Background())
	req := NewRequestWithServerStream(ss)

	assert.True(t, req.IsStream())
	assert.Panics(t, func() { req.SetPayload(nil) })
	assert.Panics(t, func() { req.GetPayload() })
	assert.Panics(t, func() { req.RawPayload() })
	assert.Panics(t, func() { req.PayloadSize() })
	assert.Panics(t, func() { req.ToBuilderRequest("") })
	assert.NotPanics(t, func() { req.Close() })
	assert.NotPanics(t, func() { req.SetSourceHost("1.1.1.1") })

}

func TestServiceAndMethod(t *testing.T) {
	assert := assert.New(t)
	fake := NewFakeServerStream(context.Background())
	request := NewRequestWithServerStream(fake)
	assert.Empty(request.FullMethod())
	request.SetFullMethod("/foo/bar")
	assert.Equal("bar", request.Method())
	assert.Equal("/foo", request.Service())

	request.SetFullMethod("foobar")
	assert.Equal("foobar", request.Method())
	assert.Equal("foobar", request.Service())
}
