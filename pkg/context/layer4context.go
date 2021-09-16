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
	"github.com/megaease/easegress/pkg/util/connection"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
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
		LocalAddr() net.Addr
		ClientAddr() net.Addr
		UpstreamAddr() net.Addr
		SetUpstreamAddr(addr net.Addr)

		GetReadBuffer() iobufferpool.IoBuffer
		AppendReadBuffer(buffer iobufferpool.IoBuffer)
		GetWriteBuffer() iobufferpool.IoBuffer
		AppendWriteBuffer(buffer iobufferpool.IoBuffer)

		WriteToClient(buffer iobufferpool.IoBuffer)
		WriteToUpstream(buffer iobufferpool.IoBuffer)

		Finish()
		Duration() time.Duration
		StopChan() chan struct{} // client connection and upstream connection stop by this chan

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

		protocol     string
		localAddr    net.Addr
		clientAddr   net.Addr
		upstreamAddr net.Addr
		stopChan     chan struct{} // notify quit read loop and write loop

		clientConn   connection.Connection
		upstreamConn connection.UpstreamConnection

		readBuffer     iobufferpool.IoBuffer
		writeBuffer    iobufferpool.IoBuffer
		connectionArgs *ConnectionArgs

		startTime *time.Time // connection accept time
		endTime   *time.Time // connection close time

		caller HandlerCaller
	}
)

// NewLayer4Context creates an Layer4Context.
func NewLayer4Context(clientConn *connection.Connection, stopChan chan struct{}) *layer4Context {

	startTime := time.Now()
	res := layer4Context{
		mutex:      sync.Mutex{},
		protocol:   clientConn.Protocol(),
		localAddr:  clientConn.LocalAddr(),
		clientAddr: clientConn.RemoteAddr(),
		stopChan:   stopChan,
		startTime:  &startTime,
	}
	return &res
}

func (ctx *layer4Context) Lock() {
	ctx.mutex.Lock()
}

func (ctx *layer4Context) Unlock() {
	ctx.mutex.Unlock()
}

// Protocol get proxy protocol
func (ctx *layer4Context) Protocol() string {
	return ctx.protocol
}

func (ctx *layer4Context) LocalAddr() net.Addr {
	return ctx.localAddr
}

func (ctx *layer4Context) ClientAddr() net.Addr {
	return ctx.ClientAddr()
}

// UpstreamAddr get upstream addr
func (ctx *layer4Context) UpstreamAddr() net.Addr {
	return ctx.upstreamAddr
}

func (ctx *layer4Context) SetUpstreamAddr(addr net.Addr) {
	ctx.upstreamAddr = addr
}

func (ctx *layer4Context) StopChan() chan struct{} {
	return ctx.stopChan
}

// GetReadBuffer get read buffer
func (ctx *layer4Context) GetReadBuffer() iobufferpool.IoBuffer {
	return ctx.readBuffer
}

// AppendReadBuffer filter receive client data, append data to ctx read buffer for other filters handle
func (ctx *layer4Context) AppendReadBuffer(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	_ = ctx.readBuffer.Append(buffer.Bytes())
}

// GetWriteBuffer get write buffer
func (ctx *layer4Context) GetWriteBuffer() iobufferpool.IoBuffer {
	return ctx.writeBuffer
}

// AppendWriteBuffer filter receive upstream data, append data to ctx write buffer for other filters handle
func (ctx *layer4Context) AppendWriteBuffer(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	_ = ctx.writeBuffer.Append(buffer.Bytes())
}

// WriteToClient filter handle client upload data, send result to upstream connection
func (ctx *layer4Context) WriteToClient(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	_ = ctx.upstreamConn.Write(buffer)
}

// WriteToUpstream filter handle client upload data, send result to upstream connection
func (ctx *layer4Context) WriteToUpstream(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	_ = ctx.clientConn.Write(buffer)
}

func (ctx *layer4Context) CallNextHandler(lastResult string) string {
	return ctx.caller(lastResult)
}

func (ctx *layer4Context) SetHandlerCaller(caller HandlerCaller) {
	ctx.caller = caller
}

func (ctx *layer4Context) Finish() {
	finish := time.Now()
	ctx.endTime = &finish
}

// Duration get context execute duration
func (ctx *layer4Context) Duration() time.Duration {
	if ctx.endTime != nil {
		return ctx.endTime.Sub(*ctx.startTime)
	}
	return time.Now().Sub(*ctx.startTime)
}
