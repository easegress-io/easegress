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

// Package grpcserver implements the GRPCServer.
package grpcserver

import (
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of GRPCServer.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of HTTPServer.
	Kind = "GRPCServer"
)

func init() {
	supervisor.Register(&GRPCServer{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"grpc"},
	})
}

type (
	// GRPCServer  is TrafficGate Object GRPCServer
	GRPCServer struct {
		runtime *runtime
	}
)

// Category returns the category of GrpcServer.
func (g *GRPCServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of GrpcServer.
func (g *GRPCServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default Spec of GrpcServer.
func (g *GRPCServer) DefaultSpec() interface{} {
	return &Spec{
		MaxConnectionIdle: "60s",
		MaxConnections:    10240,
	}
}

// Init first create GrpcServer by Spec.name
func (g *GRPCServer) Init(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	g.runtime = newRuntime(superSpec, muxMapper)

	g.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Inherit inherits previous generation of GrpcServer.
func (g *GRPCServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper context.MuxMapper) {
	g.runtime = previousGeneration.(*GRPCServer).runtime

	g.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Status returns the status of GrpcServer.
func (g *GRPCServer) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: g.runtime.Status(),
	}
}

// Close close GrpcServer
func (g *GRPCServer) Close() {
	g.runtime.Close()
}
