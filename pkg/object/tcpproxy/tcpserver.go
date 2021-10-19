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

package tcpproxy

import (
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of TCPServer.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of TCPServer.
	Kind = "TCPServer"
)

func init() {
	supervisor.Register(&TCPServer{})
}

type (
	// TCPServer is Object of tcp server.
	TCPServer struct {
		runtime *runtime
	}
)

// Category returns the category of TCPServer.
func (l4 *TCPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of TCPServer.
func (l4 *TCPServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TCPServer.
func (l4 *TCPServer) DefaultSpec() interface{} {
	return &Spec{
		MaxConnections: 1024,
		ConnectTimeout: 5 * 1000,
	}
}

// Validate validates the tcp server structure.
func (l4 *TCPServer) Validate() error {
	return nil
}

// Init initializes TCPServer.
func (l4 *TCPServer) Init(superSpec *supervisor.Spec) {

	l4.runtime = newRuntime(superSpec)
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Inherit inherits previous generation of TCPServer.
func (l4 *TCPServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {

	l4.runtime = previousGeneration.(*TCPServer).runtime
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Status is the wrapper of runtimes Status.
func (l4 *TCPServer) Status() *supervisor.Status {
	return &supervisor.Status{}
}

// Close closes TCPServer.
func (l4 *TCPServer) Close() {
	l4.runtime.Close()
}
