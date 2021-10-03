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
	"github.com/megaease/easegress/pkg/util/layer4stat"
	"net"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/util/connection"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
)

type (
	// Layer4Context is all context of an TCP processing.
	// It is not goroutine-safe, callers must use Lock/Unlock
	// to protect it by themselves.
	Layer4Context interface {
		Lock()
		Unlock()

		// Protocol current support tcp/udp, future maybe support unix
		Protocol() string
		// LocalAddr listen addr for layer4 server
		LocalAddr() net.Addr
		// ClientAddr client addr
		ClientAddr() net.Addr
		// UpstreamAddr addr for upstream server
		UpstreamAddr() net.Addr

		// GetClientReadBuffer get io buffer read from client
		GetClientReadBuffer() iobufferpool.IoBuffer
		// GetWriteToClientBuffer get io buffer write to client
		GetWriteToClientBuffer() iobufferpool.IoBuffer
		// GetUpstreamReadBuffer get io buffer read from upstream
		GetUpstreamReadBuffer() iobufferpool.IoBuffer
		// GetWriteToUpstreamBuffer get io buffer write to upstream
		GetWriteToUpstreamBuffer() iobufferpool.IoBuffer

		// WriteToClient get write to client buffer and send to client
		WriteToClient()
		// DirectWriteToClient directly write to client
		DirectWriteToClient(buffer iobufferpool.IoBuffer)
		// WriteToUpstream get write to upstream buffer and send to upstream
		WriteToUpstream()
		// DirectWriteToUpstream directly write to upstream
		DirectWriteToUpstream(buffer iobufferpool.IoBuffer)

		// StatMetric get
		StatMetric() *layer4stat.Metric

		// Finish close context and release buffer resource
		Finish()
		Duration() time.Duration

		CallNextHandler(lastResult string) string
		SetHandlerCaller(caller HandlerCaller)
	}

	layer4Context struct {
		mu       sync.Mutex
		reqSize  uint64
		respSize uint64

		protocol     string
		localAddr    net.Addr
		clientAddr   net.Addr
		upstreamAddr net.Addr

		clientConn   *connection.Connection
		upstreamConn *connection.UpstreamConnection

		clientReadBuffer    iobufferpool.IoBuffer
		clientWriteBuffer   iobufferpool.IoBuffer
		upstreamReadBuffer  iobufferpool.IoBuffer
		upstreamWriteBuffer iobufferpool.IoBuffer

		startTime *time.Time // connection accept time
		endTime   *time.Time // connection close time

		caller HandlerCaller
	}
)

// NewLayer4Context creates an Layer4Context.
// @param cliAddr udp client addr(client addr can not get from udp listen server)
func NewLayer4Context(cliConn *connection.Connection, upstreamConn *connection.UpstreamConnection, cliAddr net.Addr) *layer4Context {
	startTime := time.Now()
	ctx := &layer4Context{
		protocol:     cliConn.Protocol(),
		localAddr:    cliConn.LocalAddr(),
		upstreamAddr: upstreamConn.RemoteAddr(),
		clientConn:   cliConn,
		upstreamConn: upstreamConn,
		startTime:    &startTime,

		mu: sync.Mutex{},
	}

	if cliAddr != nil {
		ctx.clientAddr = cliAddr
	} else {
		ctx.clientAddr = cliConn.RemoteAddr() // nil for udp server conn
	}

	switch ctx.protocol {
	case "udp":
		ctx.clientReadBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
		ctx.clientWriteBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
		ctx.upstreamReadBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
		ctx.upstreamWriteBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
	case "tcp":
		ctx.clientReadBuffer = iobufferpool.GetIoBuffer(iobufferpool.DefaultBufferReadCapacity)
		ctx.clientWriteBuffer = iobufferpool.GetIoBuffer(iobufferpool.DefaultBufferReadCapacity)
		ctx.upstreamReadBuffer = iobufferpool.GetIoBuffer(iobufferpool.DefaultBufferReadCapacity)
		ctx.upstreamWriteBuffer = iobufferpool.GetIoBuffer(iobufferpool.DefaultBufferReadCapacity)
	}
	return ctx
}

// Lock acquire context lock
func (ctx *layer4Context) Lock() {
	ctx.mu.Lock()
}

// Unlock release lock
func (ctx *layer4Context) Unlock() {
	ctx.mu.Unlock()
}

// Protocol get proxy protocol
func (ctx *layer4Context) Protocol() string {
	return ctx.protocol
}

func (ctx *layer4Context) LocalAddr() net.Addr {
	return ctx.localAddr
}

func (ctx *layer4Context) ClientAddr() net.Addr {
	return ctx.clientAddr
}

// UpstreamAddr get upstream addr
func (ctx *layer4Context) UpstreamAddr() net.Addr {
	return ctx.upstreamAddr
}

func (ctx *layer4Context) StatMetric() *layer4stat.Metric {
	return &layer4stat.Metric{
		Err:      false,
		Duration: ctx.Duration(),
		ReqSize:  ctx.reqSize,
		RespSize: ctx.respSize,
	}
}

// GetClientReadBuffer get read buffer for client conn
func (ctx *layer4Context) GetClientReadBuffer() iobufferpool.IoBuffer {
	return ctx.clientReadBuffer
}

// GetWriteToClientBuffer get write buffer sync to client
func (ctx *layer4Context) GetWriteToClientBuffer() iobufferpool.IoBuffer {
	return ctx.clientWriteBuffer
}

// GetUpstreamReadBuffer get read buffer for upstream conn
func (ctx *layer4Context) GetUpstreamReadBuffer() iobufferpool.IoBuffer {
	return ctx.upstreamReadBuffer
}

// GetWriteToUpstreamBuffer get write buffer sync to upstream
func (ctx *layer4Context) GetWriteToUpstreamBuffer() iobufferpool.IoBuffer {
	return ctx.upstreamWriteBuffer
}

// WriteToClient filter handle client upload data, send result to upstream connection
func (ctx *layer4Context) WriteToClient() {
	if ctx.clientWriteBuffer == nil || ctx.upstreamWriteBuffer.Len() == 0 {
		return
	}
	buf := ctx.clientWriteBuffer.Clone()
	ctx.respSize += uint64(buf.Len())
	ctx.clientWriteBuffer.Reset()
	_ = ctx.upstreamConn.Write(buf)
}

func (ctx *layer4Context) DirectWriteToClient(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	ctx.respSize += uint64(buffer.Len())
	_ = ctx.upstreamConn.Write(buffer.Clone())
}

// WriteToUpstream filter handle client upload data, send result to upstream connection
func (ctx *layer4Context) WriteToUpstream() {
	if ctx.upstreamWriteBuffer == nil || ctx.upstreamWriteBuffer.Len() == 0 {
		return
	}
	buf := ctx.upstreamWriteBuffer.Clone()
	ctx.reqSize += uint64(buf.Len())
	_ = ctx.clientConn.Write(buf)
	ctx.upstreamWriteBuffer.Reset()
}

func (ctx *layer4Context) DirectWriteToUpstream(buffer iobufferpool.IoBuffer) {
	if buffer == nil || buffer.Len() == 0 {
		return
	}
	ctx.reqSize += uint64(buffer.Len())
	_ = ctx.clientConn.Write(buffer.Clone())
}

// CallNextHandler call handler caller
func (ctx *layer4Context) CallNextHandler(lastResult string) string {
	return ctx.caller(lastResult)
}

// SetHandlerCaller set handler caller
func (ctx *layer4Context) SetHandlerCaller(caller HandlerCaller) {
	ctx.caller = caller
}

// Finish context finish handler
func (ctx *layer4Context) Finish() {
	_ = iobufferpool.PutIoBuffer(ctx.clientReadBuffer)
	_ = iobufferpool.PutIoBuffer(ctx.clientWriteBuffer)
	_ = iobufferpool.PutIoBuffer(ctx.upstreamReadBuffer)
	_ = iobufferpool.PutIoBuffer(ctx.upstreamWriteBuffer)
	ctx.clientReadBuffer = nil
	ctx.clientWriteBuffer = nil
	ctx.upstreamReadBuffer = nil
	ctx.upstreamWriteBuffer = nil

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
