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
	"net"
	"sync"
	"time"
)

type ConnectionType uint16

const (
	DownstreamConnection ConnectionType = iota
	UpstreamConnection
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
		UpstreamAddr() net.Addr
		DownstreamAddr() net.Addr
		// SetDownstreamAddr use for udp downstream addr
		SetDownstreamAddr(addr net.Addr)
		// Finish close by downstream connection and upstream connection
		Finish(t ConnectionType)
		// Duration context alive duration
		Duration() time.Duration

		CallNextHandler(lastResult string) string
		SetHandlerCaller(caller HandlerCaller)
	}

	layer4Context struct {
		mutex sync.Mutex

		protocol       string // tcp/udp
		localAddr      net.Addr
		downstreamAddr net.Addr
		upstreamAddr   net.Addr
		startTime      *time.Time // connection accept time
		endTime        *time.Time // connection close time

		caller HandlerCaller
	}
)

// NewLayer4Context creates an Layer4Context.
func NewLayer4Context(protocol string, localAddr net.Addr, downstreamAddr, upstreamAddr net.Addr) *layer4Context {

	startTime := time.Now()
	res := layer4Context{
		mutex:          sync.Mutex{},
		protocol:       protocol,
		startTime:      &startTime,
		localAddr:      localAddr,
		downstreamAddr: downstreamAddr,
		upstreamAddr:   upstreamAddr,
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

func (ctx *layer4Context) DownstreamAddr() net.Addr {
	return ctx.downstreamAddr
}

func (ctx *layer4Context) SetDownstreamAddr(addr net.Addr) {
	ctx.downstreamAddr = addr
}

func (ctx *layer4Context) UpstreamAddr() net.Addr {
	return ctx.upstreamAddr
}

func (ctx *layer4Context) Finish(t ConnectionType) {
	finish := time.Now()
	ctx.endTime = &finish
}

func (ctx *layer4Context) Duration() time.Duration {
	if ctx.endTime != nil {
		return ctx.endTime.Sub(*ctx.startTime)
	}
	return time.Now().Sub(*ctx.startTime)
}

func (ctx *layer4Context) CallNextHandler(lastResult string) string {
	return ctx.caller(lastResult)
}

func (ctx *layer4Context) SetHandlerCaller(caller HandlerCaller) {
	ctx.caller = caller
}
