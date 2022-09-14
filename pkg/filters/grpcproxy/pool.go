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
	stdcontext "context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	grpcpool "github.com/megaease/easegress/pkg/util/connectionpool/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/megaease/easegress/pkg/context"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

// serverPoolError is the error returned by handler function of
// a server pool.
type serverPoolError struct {
	status *status.Status
	result string
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

var (
	desc = &grpc.StreamDesc{
		// we assume that client side and server side both use stream calls.
		// in test, only one or neither of the client and the server use streaming calls,
		// gRPC server works well too
		ClientStreams: true,
		ServerStreams: true,
	}
)

// ServerPool defines a server pool.
type ServerPool struct {
	proxy *Proxy
	spec  *ServerPoolSpec
	done  chan struct{}
	wg    sync.WaitGroup
	name  string

	filter                RequestMatcher
	loadBalancer          atomic.Value
	timeout               time.Duration
	circuitBreakerWrapper resilience.Wrapper
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	SpanName             string              `yaml:"spanName" jsonschema:"omitempty"`
	Filter               *RequestMatcherSpec `yaml:"filter" jsonschema:"omitempty"`
	ServerTags           []string            `yaml:"serverTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers              []*Server           `yaml:"servers" jsonschema:"omitempty"`
	ServiceRegistry      string              `yaml:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName          string              `yaml:"serviceName" jsonschema:"omitempty"`
	LoadBalance          *LoadBalanceSpec    `yaml:"loadBalance" jsonschema:"omitempty"`
	Timeout              string              `yaml:"timeout" jsonschema:"omitempty,format=duration"`
	CircuitBreakerPolicy string              `yaml:"circuitBreakerPolicy" jsonschema:"omitempty"`
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
		done:  make(chan struct{}),
		name:  name,
	}

	if spec.Filter != nil {
		sp.filter = NewRequestMatcher(spec.Filter)
	}

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		sp.createLoadBalancer(sp.spec.Servers)
	} else {
		sp.watchServers()
	}

	if spec.Timeout != "" {
		sp.timeout, _ = time.ParseDuration(spec.Timeout)
	}

	return sp
}

// LoadBalancer returns the load balancer of the server pool.
func (sp *ServerPool) LoadBalancer() LoadBalancer {
	return sp.loadBalancer.Load().(LoadBalancer)
}

func (sp *ServerPool) createLoadBalancer(servers []*Server) {
	for _, server := range servers {
		server.checkAddrPattern()
	}

	spec := sp.spec.LoadBalance
	if spec == nil {
		spec = &LoadBalanceSpec{}
	}

	lb := NewLoadBalancer(spec, servers)
	sp.loadBalancer.Store(lb)
}

func (sp *ServerPool) watchServers() {
	entity := sp.proxy.super.MustGetSystemController(serviceregistry.Kind)
	registry := entity.Instance().(*serviceregistry.ServiceRegistry)

	instances, err := registry.ListServiceInstances(sp.spec.ServiceRegistry, sp.spec.ServiceName)
	if err != nil {
		msgFmt := "first try to use service %s/%s failed(will try again): %v"
		logger.Warnf(msgFmt, sp.spec.ServiceRegistry, sp.spec.ServiceName, err)
		sp.createLoadBalancer(sp.spec.Servers)
	}

	sp.useService(instances)

	watcher := registry.NewServiceWatcher(sp.spec.ServiceRegistry, sp.spec.ServiceName)
	sp.wg.Add(1)
	go func() {
		for {
			select {
			case <-sp.done:
				watcher.Stop()
				sp.wg.Done()
				return
			case event := <-watcher.Watch():
				sp.useService(event.Instances)
			}
		}
	}()
}

func (sp *ServerPool) useService(instances map[string]*serviceregistry.ServiceInstanceSpec) {
	servers := make([]*Server, 0)

	for _, instance := range instances {
		// default to true in case of sp.spec.ServerTags is empty
		match := true

		for _, tag := range sp.spec.ServerTags {
			if match = stringtool.StrInSlice(tag, instance.Tags); match {
				break
			}
		}

		if match {
			servers = append(servers, &Server{
				URL:    instance.URL(),
				Tags:   instance.Tags,
				Weight: instance.Weight,
			})
		}
	}

	if len(servers) == 0 {
		msgFmt := "%s/%s: no service instance satisfy tags: %v"
		logger.Warnf(msgFmt, sp.spec.ServiceRegistry, sp.spec.ServiceName, sp.spec.ServerTags)
		servers = sp.spec.Servers
	}

	sp.createLoadBalancer(servers)
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
		spCtx.stdw.SetTrailer(spCtx.resp.Trailer().GetMD())
	}()

	handler := func(stdctx stdcontext.Context) error {
		if sp.timeout > 0 {
			var cancel stdcontext.CancelFunc
			stdctx, cancel = stdcontext.WithTimeout(stdctx, sp.timeout)
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
		logger.Debugf("%s: short circuited by circuit break policy", sp.name)
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

func (sp *ServerPool) doHandle(ctx stdcontext.Context, spCtx *serverPoolContext) error {
	lb := sp.LoadBalancer()
	svr := lb.ChooseServer(spCtx.req)
	// if there's no available server.
	if svr == nil {
		logger.Debugf("%s: no available server", sp.name)
		return serverPoolError{status.New(codes.InvalidArgument, "no available server"), resultClientError}
	}
	if f, ok := lb.(ReusableServerLB); ok {
		defer f.ReturnServer(svr)
	}
	// maybe be rewrite by grpcserver.MuxPath#rewrite
	fullMethodName := spCtx.req.Path()
	if fullMethodName == "" {
		return serverPoolError{status.New(codes.InvalidArgument, "unknown called method from context"), resultClientError}
	}
	send2ProviderCtx, cancelContext := stdcontext.WithCancel(metadata.NewOutgoingContext(ctx, spCtx.req.RawHeader().GetMD()))
	defer cancelContext()
	var proxyAsClientStream grpc.ClientStream
	if !sp.proxy.spec.UseConnectionPool {
		conn, err := sp.dial(spCtx.req.SourceHost(), svr.URL)
		if err != nil {
			logger.Infof("create new conn without pool fail %s for source addr %s, target addr %s, path %s",
				err.Error(), spCtx.req.SourceHost(), svr.URL, fullMethodName)
			return serverPoolError{status: status.Convert(err), result: resultInternalError}
		}
		proxyAsClientStream, err = conn.NewStream(send2ProviderCtx, desc, fullMethodName)
		if err != nil {
			conn.Close()
			logger.Infof("create new stream fail %s for source addr %s, target addr %s, path %s",
				err.Error(), spCtx.req.SourceHost(), svr.URL, fullMethodName)
			return serverPoolError{status: status.Convert(err), result: resultInternalError}
		}
	} else {
		conn, err := sp.dialWithConnPool(svr.URL)
		if err != nil {
			logger.Infof("create new conn without pool fail %s for source addr %s, target addr %s, path %s",
				err.Error(), spCtx.req.SourceHost(), svr.URL, fullMethodName)
			return serverPoolError{status: status.Convert(err), result: resultInternalError}
		}
		defer sp.proxy.pool.ReleaseConn(conn)
		proxyAsClientStream, err = conn.NewStream(send2ProviderCtx, desc, fullMethodName)
		if err != nil {
			logger.Infof("create new stream fail %s for source addr %s, target addr %s, path %s",
				err.Error(), spCtx.req.SourceHost(), svr.URL, fullMethodName)
			return serverPoolError{status: status.Convert(err), result: resultInternalError}
		}
	}
	result := sp.biTransport(spCtx, proxyAsClientStream)
	if result != nil && result != io.EOF {
		logger.Infof("create new stream fail %s for source addr %s, target addr %s, path %s",
			result.Error(), spCtx.req.SourceHost(), svr.URL, fullMethodName)
	}
	return result
}

func (sp *ServerPool) biTransport(ctx *serverPoolContext, proxyAsClientStream grpc.ClientStream) error {
	// Explicitly *do not Close* c2sErrChan and c2sErrChan, otherwise the select below will not terminate.
	// Channels do not have to be closed, it is just a control flow mechanism, see
	// https://groups.google.com/forum/#!msg/golang-nuts/pZwdYRGxCIk/qpbHxRRPJdUJ
	c2sErrChan := sp.forwardClientToServer(ctx.stdr, proxyAsClientStream)
	s2cErrChan := sp.forwardServerToClient(proxyAsClientStream, ctx.stdw, ctx.resp)
	// We don't know which side is going to stop sending first, so we need a select between the two.
	for i := 0; i < 2; i++ {
		select {
		case c2sErr := <-c2sErrChan:
			if c2sErr == io.EOF {
				// this is the happy case where the sender has encountered io.EOF, and won't be sending anymore./
				// the clientStream>serverStream may continue pumping though.
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
	return serverPoolError{status.Newf(codes.Internal, "gRPC proxying should never reach this stage."), resultInternalError}

}

func (sp *ServerPool) dial(sourceAddr, targetAddr string) (*Conn, error) {
	// avoid to meet the source addr not change but target changed.
	key := sp.connectionKey(sourceAddr, targetAddr)
	sp.proxy.locker.Lock()
	defer sp.proxy.locker.Unlock()
	if v, ok := sp.proxy.conns[key]; !ok || v == nil || v.GetState() == connectivity.Shutdown {
		t, _ := time.ParseDuration(sp.proxy.spec.ConnectTimeout)
		timeout, cancelFunc := stdcontext.WithTimeout(stdcontext.Background(), t)
		defer cancelFunc()
		dial, err := grpc.DialContext(timeout, targetAddr, grpc.WithBlock(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithCodec(grpcpool.GetCodecInstance()))
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
		}
		if old := sp.proxy.conns[key]; old != nil && !old.isClose {
			old.isClose = true
		}
		sp.proxy.conns[key] = &Conn{
			ClientConn: dial,
			key:        key,
			proxy:      sp.proxy,
			isClose:    false,
		}
	}
	return sp.proxy.conns[key], nil

}

func (sp *ServerPool) connectionKey(sourceAddr, targetAddr string) string {
	return fmt.Sprintf("%s:%s", sourceAddr, targetAddr)
}

func (sp *ServerPool) dialWithConnPool(targetAddr string) (*grpcpool.Connection, error) {
	conn, err := sp.proxy.pool.Get(targetAddr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return conn.(*grpcpool.Connection), nil
}

func (sp *ServerPool) buildOutputResponse(spCtx *serverPoolContext, s *status.Status) {
	spCtx.resp.SetStatus(s)
	spCtx.SetOutputResponse(spCtx.resp)
}

func (sp *ServerPool) close() {
	close(sp.done)
	sp.wg.Wait()
}

func (sp *ServerPool) forwardServerToClient(src grpc.ClientStream, dst grpc.ServerStream, response *grpcprot.Response) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				break
			}
			if i == 0 {
				// This is a bit of a hack, but client to server headers are only readable after first client msg is
				// received but must be written to server stream before the first msg is flushed.
				// This is the only place to do it nicely.
				md, err := src.Header()
				if err != nil {
					ret <- err
					break
				}
				md = metadata.Join(response.RawHeader().GetMD(), md)

				if err := dst.SendHeader(md); err != nil {
					ret <- err
					break
				}
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}

func (sp *ServerPool) forwardClientToServer(src grpc.ServerStream, dst grpc.ClientStream) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				break
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}
