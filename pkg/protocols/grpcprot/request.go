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
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type (
	// Request wraps grpc.ServerStream,based on the following considerations:
	// 1.in the case of replication, the dst object should be consistent with the source object,
	// so all modifications should be written directly into source object's ctx
	// 2.the atomicity of header.
	Request struct {
		stream   grpc.ServerStream
		ctx      context.Context
		cancel   context.CancelFunc
		header   *Header
		headerMx *sync.Mutex
		realIP   string
		sts      *serverTransportStream
		peer     *peer.Peer
		host     string
	}

	serverTransportStream struct {
		grpc.ServerTransportStream
		path string
	}
	// Addr is a net.Addr backed by either a TCP "ip:port" string, or
	// the empty string if unknown.
	Addr struct {
		addr string
	}
)

const (
	// Authority is the key of authority in grpc metadata
	Authority = ":authority"
)

var (
	_ protocols.Request = (*Request)(nil)
)

// NewRequestWithServerStream creates a new request from a grpc.ServerStream
// that have data and conn, so it could reply to client with data
func NewRequestWithServerStream(stream grpc.ServerStream) *Request {
	r := NewRequestWithContext(stream.Context())
	r.stream = stream
	return r
}

// NewRequestWithContext creates a new request from context.
// that only have request data, not support reply to client
func NewRequestWithContext(ctx context.Context) *Request {
	r := &Request{
		ctx:      ctx,
		headerMx: &sync.Mutex{},
	}
	r.ctx, r.cancel = context.WithCancel(r.ctx)

	r.sts = &serverTransportStream{ServerTransportStream: grpc.ServerTransportStreamFromContext(r.ctx)}
	// grpc's method equals request.path in http standard lib
	if method, ok := grpc.Method(r.ctx); ok {
		r.sts.path = method
	}
	r.ctx = grpc.NewContextWithServerTransportStream(r.ctx, r.sts)
	md, ok := metadata.FromIncomingContext(r.ctx)
	if !ok {
		logger.Infof("couldn't get headers' md from grpc context")
		md = metadata.New(nil)
	}

	r.header = NewHeader(md)
	r.ctx = metadata.NewIncomingContext(r.ctx, r.header.md)
	r.peer = &peer.Peer{Addr: &Addr{}}

	if peerInfo, ok := peer.FromContext(r.ctx); ok {
		r.peer.Addr.(*Addr).setAddr(peerInfo.Addr.String())
		r.peer.AuthInfo = peerInfo.AuthInfo
	}
	r.ctx = peer.NewContext(r.ctx, r.peer)

	return r
}

// Network returns the network type of the address.
func (a *Addr) Network() string {
	if a != nil {
		// Per the documentation on net/http.Request.RemoteAddr, if this is
		// set, it's set to the IP:port of the peer (hence, TCP):
		// https://golang.org/pkg/net/http/#Request
		//
		// If we want to support Unix sockets later, we can
		// add our own grpc-specific convention within the
		// grpc codebase to set RemoteAddr to a different
		// format, or probably better: we can attach it to the
		// context and use that from serverHandlerTransport.RemoteAddr.
		return "tcp"
	}
	return ""
}

// String implements the Stringer interface.
func (a *Addr) String() string { return a.addr }

func (a *Addr) setAddr(addr string) {
	a.addr = addr
}

func (sts *serverTransportStream) setMethod(path string) {
	sts.path = path
}

// GetServerStream return the underlying grpc.ServerStream
func (r *Request) GetServerStream() grpc.ServerStream {
	return r.stream
}

// SetHeader use md set Request.header
func (r *Request) SetHeader(header *Header) {
	r.headerMx.Lock()
	defer r.headerMx.Unlock()
	r.header.md = header.md
	r.ctx = metadata.NewIncomingContext(r.ctx, r.header.md)
}

// Method returns full method name of the grpc request.
// The returned string is in the format of "/service/method"
// override grpc.ServerTransportStream
func (sts *serverTransportStream) Method() string {
	return sts.path
}

// IsStream returns whether the payload of the request is a stream.
func (r *Request) IsStream() bool {
	return true
}

// SetPayload set the payload of the response to payload. The payload
// could be a string, a byte slice, or an io.Reader, and if it is an
// io.Reader, it will be treated as a stream, if this is not desired,
// please read the data to a byte slice, and set the byte slice as
// the payload.
func (r *Request) SetPayload(payload interface{}) {
	panic("implement me")
}

// GetPayload returns a payload reader. For non-stream payload, the
// returned reader is always a new one, which contains the full data.
// For stream payload, the function always returns the same reader.
func (r *Request) GetPayload() io.Reader {
	panic("implement me")
}

// RawPayload returns the payload in []byte, the caller should not
// modify its content. The function panic if the payload is a stream.
func (r *Request) RawPayload() []byte {
	panic("the payload is a large one")
}

// PayloadSize returns the size of the payload. If the payload is a
// stream, it returns the bytes count that have been currently read
// out.
func (r *Request) PayloadSize() int64 {
	panic("implement me")
}

// ToBuilderRequest wraps the request and returns the wrapper, the
// return value can be used in the template of the Builder filters.
func (r *Request) ToBuilderRequest(name string) interface{} {
	panic("implement me")
}

// Close returns the request
func (r *Request) Close() {
	r.cancel()
}

// Context returns the context of the request
func (r *Request) Context() context.Context {
	return r.ctx
}

// FullMethod returns full method name of the grpc request.
// The returned string is in the format of "/service/method"
func (r *Request) FullMethod() string {
	return r.sts.path
}

// Method returns method name of the grpc request.
func (r *Request) Method() string {
	if index := strings.LastIndex(r.sts.path, "/"); index != -1 {
		return r.sts.path[index+1:]
	}
	return r.sts.path
}

// Service returns service name of the grpc request.
func (r *Request) Service() string {
	if index := strings.LastIndex(r.sts.path, "/"); index != -1 {
		return r.sts.path[0:index]
	}
	return r.sts.path
}

// SetFullMethod sets full method name of the grpc request.
func (r *Request) SetFullMethod(path string) {
	r.sts.setMethod(path)
}

// RealIP return the client ip from peer.FromContext()
func (r *Request) RealIP() string {
	ip := r.realIP
	if ip != "" {
		return ip
	}
	if strings.ContainsRune(r.peer.Addr.(*Addr).String(), ':') {
		ip, _, _ = net.SplitHostPort(r.peer.Addr.(*Addr).String())
	} else {
		ip = r.peer.Addr.(*Addr).String()
	}
	r.realIP = ip
	return ip
}

// SetRealIP set the client ip of the request.
func (r *Request) SetRealIP(ip string) {
	r.realIP = ""
	if strings.ContainsRune(r.peer.Addr.String(), ':') {
		_, port, _ := net.SplitHostPort(r.peer.Addr.String())
		r.peer.Addr.(*Addr).setAddr(fmt.Sprintf("%s:%s", ip, port))
	} else {
		r.peer.Addr.(*Addr).setAddr(ip)
	}
}

// SourceHost returns peer host from grpc context
func (r *Request) SourceHost() string {
	return r.peer.Addr.(*Addr).String()
}

// SetSourceHost set the source host of the request.
func (r *Request) SetSourceHost(sourceHost string) {
	r.peer.Addr.(*Addr).setAddr(sourceHost)
}

// Host refer to google.golang.org\grpc@v1.46.2\internal\transport\http2_server.go#operateHeaders
// It may be of the form "host:port" or contain an international domain name.
func (r *Request) Host() string {
	return r.header.GetFirst(Authority)
}

// OnlyHost return host or hostname without port
func (r *Request) OnlyHost() string {
	if r.host != "" {
		return r.host
	}
	host := r.Host()
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	r.host = host
	return host
}

// SetHost set the host of the request.
func (r *Request) SetHost(host string) {
	r.header.Set(Authority, host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		r.host = h
	} else {
		r.host = host
	}
}

// Header returns the header of the request in type protocols.Header.
func (r *Request) Header() protocols.Header {
	return r.header
}

// RawHeader returns the header of the request in type metadata.MD.
func (r *Request) RawHeader() *Header {
	return r.header
}
