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

package context

import (
	"bytes"
	stdcontext "context"
	"github.com/megaease/easegress/pkg/object/layer4rawserver"
	"github.com/megaease/easegress/pkg/util/connectionwrapper"
	"net"
	"sync"
	"time"
)

type (
	// Layer4Context is all context of an TCP processing.
	// It is not goroutine-safe, callers must use Lock/Unlock
	// to protect it by themselves.
	Layer4Context interface {
		Lock()
		Unlock()

		Protocol() string
		ConnectionArgs() *ConnectionArgs
		SetConnectionArgs(args *ConnectionArgs)
		LocalAddr() net.Addr
		SetLocalAddr(addr net.Addr)
		RemoteAddr() net.Addr
		SetRemoteAddr(addr net.Addr)

		Stop()

		stdcontext.Context
		Cancel(err error)
		Cancelled() bool
		ClientDisconnected() bool

		ClientConn() *connectionwrapper.Connection

		Duration() time.Duration // For log, sample, etc.
		OnFinish(func())         // For setting final client statistics, etc.
		AddTag(tag string)       // For debug, log, etc.

		Finish()

		CallNextHandler(lastResult string) string
		SetHandlerCaller(caller HandlerCaller)
	}

	ConnectionArgs struct {
		TCPNodelay        bool
		Linger            bool
		SendBufSize       uint32
		RecvBufSize       uint32
		ProxyTimeout      uint32
		ProxyReadTimeout  int64 // connection read timeout(milliseconds)
		ProxyWriteTimeout int64 // connection write timeout(milliseconds)

		startOnce sync.Once // make sure read loop and write loop start only once
	}

	layer4Context struct {
		mutex sync.Mutex

		protocol   string
		localAddr  net.Addr
		remoteAddr net.Addr
		clientConn *connectionwrapper.Connection

		connectionArgs *ConnectionArgs

		readBuffer      bytes.Buffer
		writeBuffers    net.Buffers
		ioBuffers       []bytes.Buffer
		writeBufferChan chan *[]bytes.Buffer
		stopChan        chan struct{} // notify quit read loop and write loop

		startTime *time.Time // connection accept time
		endTime   *time.Time // connection close time

		caller HandlerCaller
	}
)

// NewLayer4Context creates an Layer4Context.
func NewLayer4Context(protocol string, conn *connectionwrapper.Connection, mux *layer4rawserver.Mux) *layer4Context {

	// TODO add mux for mux mapper

	startTime := time.Now()
	res := layer4Context{
		protocol:   protocol,
		clientConn: conn,
		localAddr:  conn.Conn.LocalAddr(),
		remoteAddr: conn.Conn.RemoteAddr(),

		startTime: &startTime,
		stopChan:  make(chan struct{}),
		mutex:     sync.Mutex{},
	}
	return &res
}

func (ctx *layer4Context) Protocol() string {
	return ctx.protocol
}

func (ctx *layer4Context) ConnectionArgs() *ConnectionArgs {
	return ctx.connectionArgs
}

func (ctx *layer4Context) SetConnectionArgs(args *ConnectionArgs) {
	ctx.connectionArgs = args
}

func (ctx *layer4Context) LocalAddr() net.Addr {
	return ctx.localAddr
}

func (ctx *layer4Context) SetLocalAddr(localAddr net.Addr) {
	ctx.localAddr = localAddr
}

func (ctx *layer4Context) RemoteAddr() net.Addr {
	return ctx.remoteAddr
}

func (ctx *layer4Context) SetRemoteAddr(addr net.Addr) {
	ctx.remoteAddr = addr
}

func (ctx *layer4Context) Stop() {
	endTime := time.Now()
	ctx.endTime = &endTime

	// TODO add stat for context
}

func (ctx *layer4Context) Deadline() (deadline time.Time, ok bool) {
	panic("implement me")
}

func (ctx *layer4Context) Done() <-chan struct{} {
	panic("implement me")
}

func (ctx *layer4Context) Err() error {
	panic("implement me")
}

func (ctx *layer4Context) Value(key interface{}) interface{} {
	panic("implement me")
}

func (ctx *layer4Context) Cancel(err error) {
	panic("implement me")
}

func (ctx *layer4Context) Cancelled() bool {
	panic("implement me")
}

func (ctx *layer4Context) ClientDisconnected() bool {
	panic("implement me")
}

func (ctx *layer4Context) ClientConn() *connectionwrapper.Connection {
	return ctx.clientConn
}

func (ctx *layer4Context) OnFinish(f func()) {
	panic("implement me")
}

func (ctx *layer4Context) AddTag(tag string) {
	panic("implement me")
}

func (ctx *layer4Context) Finish() {
	panic("implement me")
}

func (ctx *layer4Context) Lock() {
	ctx.mutex.Lock()
}

func (ctx *layer4Context) Unlock() {
	ctx.mutex.Unlock()
}

func (ctx *layer4Context) CallNextHandler(lastResult string) string {
	return ctx.caller(lastResult)
}

func (ctx *layer4Context) SetHandlerCaller(caller HandlerCaller) {
	ctx.caller = caller
}

func (ctx *layer4Context) Duration() time.Duration {
	if ctx.endTime != nil {
		return ctx.endTime.Sub(*ctx.startTime)
	}

	return time.Now().Sub(*ctx.startTime)
}
