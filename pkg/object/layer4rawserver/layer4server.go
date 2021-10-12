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

package layer4rawserver

import (
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of Layer4Server.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of Layer4Server.
	Kind = "Layer4Server"
)

func init() {
	supervisor.Register(&Layer4Server{})
}

type (
	// Layer4Server is Object of tpc/udp server.
	Layer4Server struct {
		runtime *runtime
	}
)

// Category returns the category of Layer4Server.
func (l4 *Layer4Server) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Layer4Server.
func (l4 *Layer4Server) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Layer4Server.
func (l4 *Layer4Server) DefaultSpec() interface{} {
	return &Spec{
		MaxConnections: 10240,
		ConnectTimeout: 5 * 1000,
	}
}

// Validate validates the layer4 server structure.
func (l4 *Layer4Server) Validate() error {
	return nil
}

// Init initializes Layer4Server.
func (l4 *Layer4Server) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {

	l4.runtime = newRuntime(superSpec, muxMapper)
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Inherit inherits previous generation of Layer4Server.
func (l4 *Layer4Server) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {

	l4.runtime = previousGeneration.(*Layer4Server).runtime
	l4.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Status is the wrapper of runtimes Status.
func (l4 *Layer4Server) Status() *supervisor.Status {
	return &supervisor.Status{}
}

// Close closes Layer4Server.
func (l4 *Layer4Server) Close() {
	l4.runtime.Close()
}
