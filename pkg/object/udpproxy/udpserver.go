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

package udpproxy

import (
	"net"
	"sync"

	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of TCPServer.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of TCPServer.
	Kind = "UDPServer"
)

func init() {
	supervisor.Register(&UDPServer{})
}

type (
	// UDPServer is Object of udp server.
	UDPServer struct {
		runtime *runtime
	}

	connPool struct {
		pool map[string]net.Conn
		mu   sync.RWMutex
	}
)

// Category get object category
func (u *UDPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind get object kind
func (u *UDPServer) Kind() string {
	return Kind
}

// DefaultSpec get default spec of UDPServer
func (u *UDPServer) DefaultSpec() interface{} {
	return &Spec{}
}

// Status get UDPServer status
func (u *UDPServer) Status() *supervisor.Status {
	return &supervisor.Status{}
}

// Close actually close runtime
func (u *UDPServer) Close() {
	u.runtime.close()
}

// Init initializes UDPServer.
func (u *UDPServer) Init(superSpec *supervisor.Spec) {
	u.runtime = newRuntime(superSpec)
}

// Inherit inherits previous generation of UDPServer.
func (u *UDPServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {

	u.runtime = previousGeneration.(*UDPServer).runtime
	u.runtime.close()
	u.Init(superSpec)
}

func newConnPool() *connPool {
	return &connPool{
		pool: make(map[string]net.Conn),
	}
}

func (c *connPool) get(addr string) net.Conn {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pool[addr]
}

func (c *connPool) put(addr string, conn net.Conn) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.pool[addr] = conn
}

func (c *connPool) close() {
	if c == nil {
		return
	}

	for _, conn := range c.pool {
		_ = conn.Close()
	}
	c.pool = nil
}
