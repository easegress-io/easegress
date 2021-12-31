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

package trafficcontroller

import (
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

type ( // Namespace is the namespace
	Namespace struct {
		namespace string
		// The scenario here satisfies the first common case:
		// When the entry for a given key is only ever written once but read many times.
		// Reference: https://golang.org/pkg/sync/#Map
		// types of both: map[string]*supervisor.ObjectEntity
		servers   map[context.Protocol]*sync.Map
		pipelines map[context.Protocol]*sync.Map
	}

	namespaceField string
)

const (
	serverField   namespaceField = "server"
	pipelineField namespaceField = "pipeline"
)

func newNamespace(namespace string) *Namespace {
	ns := &Namespace{
		namespace: namespace,
		servers:   make(map[context.Protocol]*sync.Map),
		pipelines: make(map[context.Protocol]*sync.Map),
	}
	for _, p := range context.Protocols {
		ns.servers[p] = &sync.Map{}
		ns.pipelines[p] = &sync.Map{}
	}
	return ns
}

// GetHandler gets handler within the namespace
func (ns *Namespace) GetHandler(protocolType context.Protocol, name string) (protocol.Handler, bool) {
	pipelines, exists := ns.pipelines[protocolType]
	if !exists {
		return nil, false
	}

	entity, exists := pipelines.Load(name)
	if !exists {
		return nil, false
	}

	handler := entity.(*supervisor.ObjectEntity).Instance().(protocol.Handler)
	return handler, true
}

func (ns *Namespace) getField(name namespaceField) map[context.Protocol]*sync.Map {
	if name == serverField {
		return ns.servers
	}
	return ns.pipelines
}
