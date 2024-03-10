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

package grpcproxy

import (
	stdcontext "context"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/util/objectpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/resilience"
)

type (
	// serverPoolError is the error returned by handler function of
	// a server pool.
	serverPoolError struct {
		status *status.Status
		result string
	}
	// MultiPool manage multi Pool.
	MultiPool struct {
		pools sync.Map
		lock  sync.Mutex
		spec  *objectpool.Spec
	}
)

// NewMultiWithSpec return a new MultiPool
func NewMultiWithSpec(spec *objectpool.Spec) *MultiPool {
	return &MultiPool{spec: spec}
}

func (m *MultiPool) Put(key string, obj objectpool.PoolObject) {
	if value, ok := m.pools.Load(key); ok {
		value.(*objectpool.Pool).Put(obj)
	}
}

// Get returns an object from the pool,
//
// if there's an exists single, it will try to get an available object;
// if there's no exists single, it will create a one and try to get an available object;
func (m *MultiPool) Get(key string, ctx stdcontext.Context, new objectpool.CreateObjectFn) (objectpool.PoolObject, error) {
	var value interface{}
	var ok bool
	if value, ok = m.pools.Load(key); !ok {
		m.lock.Lock()
		if value, ok = m.pools.Load(key); !ok {
			value = objectpool.NewWithSpec(m.spec)
			m.pools.Store(key, value)
		}
		m.lock.Unlock()
	}
	return value.(*objectpool.Pool).Get(ctx, new)
}

// Error implements error.
func (spe serverPoolError) Error() string {
	return fmt.Sprintf("server pool error, status code=%+v, result=%s", spe.status, spe.result)
}

// Result returns the result string.
func (spe serverPoolError) Result() string {
	return spe.result
}

func (spe serverPoolError) Code() int {
	return int(spe.status.Proto().GetCode())
}

// serverPoolContext records the context information in calling the
// handler function.
type serverPoolContext struct {
	*context.Context
	// separate request data and connection
	// hope this design makes it clearer
	stdr grpc.ServerStream
	stdw grpc.ServerStream
	// req is just request data
	req *grpcprot.Request
	// resp is just response data
	resp *grpcprot.Response
}

var desc = &grpc.StreamDesc{
	// we assume that client side and server side both use stream calls.
	// in test, only one or neither of the client and the server use streaming calls,
	// gRPC server works well too
	ClientStreams: true,
	ServerStreams: true,
}

// ServerPool defines a server pool.
type ServerPool struct {
	BaseServerPool

	proxy *Proxy
	spec  *ServerPoolSpec

	filter                RequestMatcher
	circuitBreakerWrapper resilience.Wrapper
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	BaseServerPoolSpec `json:",inline"`

	SpanName             string              `json:"spanName,omitempty"`
	Filter               *RequestMatcherSpec `json:"filter,omitempty"`
	CircuitBreakerPolicy string              `json:"circuitBreakerPolicy,omitempty"`
}

// Validate validates ServerPoolSpec.
func (sps *ServerPoolSpec) Validate() error {
	if sps.ServiceName == "" && len(sps.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range sps.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(sps.Servers) {
		msgFmt := "not all servers have weight(%d/%d)"
		return fmt.Errorf(msgFmt, serversGotWeight, len(sps.Servers))
	}
	return nil
}

// NewServerPool creates a new server pool according to spec.
func NewServerPool(proxy *Proxy, spec *ServerPoolSpec, name string) *ServerPool {
	sp := &ServerPool{
		proxy: proxy,
		spec:  spec,
	}

	if spec.Filter != nil {
		sp.filter = NewRequestMatcher(spec.Filter)
	}

	sp.BaseServerPool.Init(sp, proxy.super, name, &spec.BaseServerPoolSpec)

	return sp
}

// CreateLoadBalancer creates a load balancer according to spec.
func (sp *ServerPool) CreateLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	if spec.Policy == "forward" {
		return newForwardLoadBalancer(spec)
	}

	lb := proxies.NewGeneralLoadBalancer(spec, servers)
	lb.Init(nil, nil, nil)
	return lb
}

// InjectResiliencePolicy injects resilience policies to the server pool.
func (sp *ServerPool) InjectResiliencePolicy(policies map[string]resilience.Policy) {
	name := sp.spec.CircuitBreakerPolicy
	if name != "" {
		p := policies[name]
		if p == nil {
			panic(fmt.Errorf("circuitbreaker policy %s not found", name))
		}
		policy, ok := p.(*resilience.CircuitBreakerPolicy)
		if !ok {
			panic(fmt.Errorf("policy %s is not a circuitBreaker policy", name))
		}
		sp.circuitBreakerWrapper = policy.CreateWrapper()
	}
}

func (sp *ServerPool) handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*grpcprot.Request)
	spCtx := &serverPoolContext{
		Context: ctx,
		req:     grpcprot.NewRequestWithContext(req.Context()),
		stdr:    req.GetServerStream(),
		stdw:    req.GetServerStream(),
		resp:    grpcprot.NewResponse(),
	}
	defer func() {
		spCtx.stdw.SetTrailer(spCtx.resp.RawTrailer().GetMD())
	}()

	handler := func(stdctx stdcontext.Context) error {
		if sp.proxy.timeout > 0 {
			var cancel stdcontext.CancelFunc
			stdctx, cancel = stdcontext.WithTimeout(stdctx, sp.proxy.timeout)
			defer cancel()
		}

		err := sp.doHandle(stdctx, spCtx)
		if err == nil {
			return nil
		}

		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("status code: %d", err.(serverPoolError).Code())
		})

		return err
	}

	if sp.circuitBreakerWrapper != nil {
		handler = sp.circuitBreakerWrapper.Wrap(handler)
	}

	// call the handler.
	err := handler(spCtx.req.Context())
	if err == nil {
		spCtx.Context.SetOutputResponse(spCtx.resp)
		return ""
	}

	// CircuitBreaker is the most outside resiliencer, if the error
	// is ErrShortCircuited, we are sure the response is nil.
	if err == resilience.ErrShortCircuited {
		logger.Debugf("%s: short circuited by circuit break policy", sp.Name)
		spCtx.AddTag("short circuited")
		sp.buildOutputResponse(spCtx, status.Newf(codes.Unavailable, "short circuited by circuit break policy"))
		return resultShortCircuited
	}

	// The error must be a serverPoolError now, we need to build a
	// response in most cases, but for failure status codes, the
	// response is already there.
	if spe, ok := err.(serverPoolError); ok {
		sp.buildOutputResponse(spCtx, spe.status)
		return spe.Result()
	}

	panic(fmt.Errorf("should not reach here"))
}

func (sp *ServerPool) getTarget(rawTarget string) string {
	target := rawTarget
	// gRPC only support ip:port, but proxies.Server.URL include scheme and forward.Key too
	if parse, err := url.Parse(target); err == nil && parse.Host != "" {
		target = parse.Host
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return ""
	}
	return target
}

func (sp *ServerPool) doHandle(ctx stdcontext.Context, spCtx *serverPoolContext) error {
	lb := sp.LoadBalancer()
	svr := lb.ChooseServer(spCtx.req)
	// if there's no available server.
	if svr == nil {
		logger.Debugf("%s: no available server", sp.Name)
		return serverPoolError{status.New(codes.InvalidArgument, "no available server"), resultClientError}
	}
	target := sp.getTarget(svr.URL)
	lb.ReturnServer(svr, spCtx.req, spCtx.resp)
	if target == "" {
		logger.Debugf("request %v from %v context target address %s invalid", spCtx.req.FullMethod(), spCtx.req.RealIP(), target)
		return serverPoolError{status.New(codes.Internal, "server url invalid"), resultInternalError}
	}

	// maybe be rewritten by grpcserver.MuxPath#rewrite
	fullMethodName := spCtx.req.FullMethod()
	if fullMethodName == "" {
		return serverPoolError{status.New(codes.InvalidArgument, "unknown called method from context"), resultClientError}
	}

	borrowCtx, cancel := stdcontext.WithCancel(stdcontext.Background())
	if sp.proxy.borrowTimeout != 0 {
		borrowCtx, cancel = stdcontext.WithTimeout(borrowCtx, sp.proxy.borrowTimeout)
	}
	defer cancel()
	conn, err := sp.proxy.connectionPool.Get(target, borrowCtx, func() (objectpool.PoolObject, error) {
		dialCtx, dialCancel := stdcontext.WithCancel(stdcontext.Background())
		if sp.proxy.connectTimeout != 0 {
			dialCtx, dialCancel = stdcontext.WithTimeout(dialCtx, sp.proxy.connectTimeout)
		}
		defer dialCancel()
		conn, err := grpc.DialContext(dialCtx, target, defaultDialOpts...)
		if err != nil {
			logger.Infof("create new grpc client connection for %s fail %v", target, err)
			return nil, err
		}
		return &clientConnWrapper{conn}, nil
	})
	if err != nil {
		logger.Infof("get connection from pool fail %s for source addr %s, target addr %s, path %s",
			err.Error(), spCtx.req.SourceHost(), target, fullMethodName)
		return serverPoolError{status: status.Convert(err), result: resultInternalError}
	}
	send2ProviderCtx, cancelContext := stdcontext.WithCancel(metadata.NewOutgoingContext(ctx, spCtx.req.RawHeader().GetMD()))
	defer cancelContext()

	proxyAsClientStream, err := conn.(*clientConnWrapper).NewStream(send2ProviderCtx, desc, fullMethodName)
	sp.proxy.connectionPool.Put(target, conn)
	if err != nil {
		logger.Infof("create new stream fail %s for source addr %s, target addr %s, path %s",
			err.Error(), spCtx.req.SourceHost(), target, fullMethodName)
		return serverPoolError{status: status.Convert(err), result: resultInternalError}
	}

	result := sp.biTransport(spCtx, proxyAsClientStream)
	if result != nil && result != io.EOF {
		logger.Infof("create new stream fail %s for source addr %s, target addr %s, path %s",
			result.Error(), spCtx.req.SourceHost(), target, fullMethodName)
	}
	return result
}

func (sp *ServerPool) biTransport(ctx *serverPoolContext, proxyAsClientStream grpc.ClientStream) error {
	// Explicitly *do not Close* c2sErrChan and c2sErrChan, otherwise the select below will not terminate.
	// Channels do not have to be closed, it is just a control flow mechanism, see
	// https://groups.google.com/forum/#!msg/golang-nuts/pZwdYRGxCIk/qpbHxRRPJdUJ
	c2sErrChan := sp.forwardS2C(ctx.stdr, proxyAsClientStream, nil)
	s2cErrChan := sp.forwardC2S(proxyAsClientStream, ctx.stdw, ctx.resp.RawHeader())
	// We don't know which side is going to stop sending first, so we need a select between the two.
	for {
		select {
		case c2sErr := <-c2sErrChan:
			if c2sErr == io.EOF {
				// this is the happy case where the sender has encountered io.EOF, and won't be sending anymore./
				// the server --> client may continue pumping though.
				proxyAsClientStream.CloseSend()
			} else {
				// however, we may have gotten a receive error (stream disconnected, a read error etc) in which case we need
				// to cancel the clientStream to the backend, let all of its goroutines be freed up by the CancelFunc and
				// exit with an error to the stack
				return serverPoolError{status.Convert(c2sErr), resultServerError}
			}
		case s2cErr := <-s2cErrChan:
			// This happens when the clientStream has nothing else to offer (io.EOF), returned a gRPC error. In those two
			// cases we may have received Trailers as part of the call. In case of other errors (stream closed) the trailers
			// will be nil.
			ctx.resp.SetTrailer(grpcprot.NewTrailer(proxyAsClientStream.Trailer()))
			// c2sErr will contain RPC error from client code. If not io.EOF return the RPC error as server stream error.
			if s2cErr != io.EOF {
				return serverPoolError{status.Convert(s2cErr), resultServerError}
			}
			return nil
		}
	}
}

func (sp *ServerPool) buildOutputResponse(spCtx *serverPoolContext, s *status.Status) {
	spCtx.resp.SetStatus(s)
	spCtx.SetOutputResponse(spCtx.resp)
}

func (sp *ServerPool) forwardC2S(src grpc.ClientStream, dst grpc.ServerStream, header *grpcprot.Header) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				return
			}
			if i == 0 {
				// This is a bit of a hack, but client to server headers are only readable after first client msg is
				// received but must be written to server stream before the first msg is flushed.
				// This is the only place to do it nicely.
				md, err := src.Header()
				if err != nil {
					ret <- err
					return
				}
				if header != nil {
					md = metadata.Join(header.GetMD(), md)
				}

				if err = dst.SendHeader(md); err != nil {
					ret <- err
					return
				}
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				return
			}
		}
	}()
	return ret
}

func (sp *ServerPool) forwardS2C(src grpc.ServerStream, dst grpc.ClientStream, header *grpcprot.Header) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				return
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				return
			}
		}
	}()
	return ret
}
