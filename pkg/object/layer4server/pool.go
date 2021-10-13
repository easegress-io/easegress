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

package layer4server

import (
	"reflect"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	pool struct {
		rules atomic.Value
	}

	// pool backend server pool
	poolRules struct {
		spec *PoolSpec

		tagPrefix string
		servers   *servers
	}
)

func newPool(super *supervisor.Supervisor, spec *PoolSpec, tagPrefix string) *pool {
	p := &pool{}

	p.rules.Store(&poolRules{
		spec: spec,

		tagPrefix: tagPrefix,
		servers:   newServers(super, spec),
	})
	return p
}

func (p *pool) next(cliAddr string) (*Server, error) {
	rules := p.rules.Load().(*poolRules)
	return rules.servers.next(cliAddr)
}

func (p *pool) close() {
	if old := p.rules.Load(); old != nil {
		oldPool := old.(*poolRules)
		oldPool.servers.close()
	}
}

func (p *pool) reloadRules(super *supervisor.Supervisor, spec *PoolSpec, tagPrefix string) {
	old := p.rules.Load().(*poolRules)
	if reflect.DeepEqual(old.spec, spec) {
		return
	}
	p.close()
	p.rules.Store(&poolRules{
		spec: spec,

		tagPrefix: tagPrefix,
		servers:   newServers(super, spec),
	})
}
