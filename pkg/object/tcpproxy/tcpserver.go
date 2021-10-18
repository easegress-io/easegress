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
	// Category is the category of TcpServer.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of TcpServer.
	Kind = "TcpServer"
)

func init() {
	supervisor.Register(&TcpServer{})
}

type (
	// TcpServer is Object of tcp server.
	TcpServer struct {
		runtime *runtime
	}
)

// Category returns the category of TcpServer.
func (l4 *TcpServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of TcpServer.
func (l4 *TcpServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TcpServer.
func (l4 *TcpServer) DefaultSpec() interface{} {
	return &Spec{
		MaxConnections: 1024,
		ConnectTimeout: 5 * 1000,
	}
}

// Validate validates the tcp server structure.
func (l4 *TcpServer) Validate() error {
	return nil
}

// Init initializes TcpServer.
func (l4 *TcpServer) Init(superSpec *supervisor.Spec) {

	l4.runtime = newRuntime(superSpec)
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Inherit inherits previous generation of TcpServer.
func (l4 *TcpServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {

	l4.runtime = previousGeneration.(*TcpServer).runtime
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Status is the wrapper of runtimes Status.
func (l4 *TcpServer) Status() *supervisor.Status {
	return &supervisor.Status{}
}

// Close closes TcpServer.
func (l4 *TcpServer) Close() {
	l4.runtime.Close()
}
