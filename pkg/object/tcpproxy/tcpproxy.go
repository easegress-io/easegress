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

// Package tcpproxy implements the TCPProxy controller.
package tcpproxy

import (
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of TCPProxy.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of TCPProxy.
	Kind = "TCPProxy"
)

func init() {
	supervisor.Register(&TCPProxy{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"tcpproxies", "tcp"},
	})
}

type (
	// TCPProxy is Object TCPProxy.
	TCPProxy struct {
		runtime *runtime
	}
)

// Category returns the category of TCPProxy.
func (tp *TCPProxy) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of TCPProxy.
func (tp *TCPProxy) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TCPProxy.
func (tp *TCPProxy) DefaultSpec() interface{} {
	return &Spec{
		Port:           8080,
		MaxConnections: 10240,
		ConnectTimeout: "10s",
		SendTimeout:    "30s",
		ReceiveTimeout: "30s",
		IdleTimeout:    "60s",
		LoadBalance:    LoadBalanceRoundRobin,
	}
}

// Init initializes TCPProxy.
func (tp *TCPProxy) Init(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	tp.runtime = newRuntime(superSpec)

	tp.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Inherit inherits previous generation of TCPProxy.
func (tp *TCPProxy) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper context.MuxMapper) {
	tp.runtime = previousGeneration.(*TCPProxy).runtime

	tp.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
	}
}

// Status is the wrapper of runtime's Status.
func (tp *TCPProxy) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: tp.runtime.Status(),
	}
}

// Close closes TCPProxy.
func (tp *TCPProxy) Close() {
	tp.runtime.Close()
}
