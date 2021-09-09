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

package upstream

import (
	"fmt"
	"net"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/layer4stat"
	"github.com/megaease/easegress/pkg/util/memorycache"
)

type (
	protocol string

	pool struct {
		spec *PoolSpec

		tagPrefix string

		servers    *servers
		layer4stat *layer4stat.Layer4Stat
	}

	// PoolSpec describes a pool of servers.
	PoolSpec struct {
		Protocol        protocol          `yaml:"protocol" jsonschema:"required" `
		SpanName        string            `yaml:"spanName" jsonschema:"omitempty"`
		ServersTags     []string          `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		Servers         []*Server         `yaml:"servers" jsonschema:"omitempty"`
		ServiceRegistry string            `yaml:"serviceRegistry" jsonschema:"omitempty"`
		ServiceName     string            `yaml:"serviceName" jsonschema:"omitempty"`
		LoadBalance     *LoadBalance      `yaml:"loadBalance" jsonschema:"required"`
		MemoryCache     *memorycache.Spec `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
	}

	// PoolStatus is the status of Pool.
	PoolStatus struct {
		Stat *layer4stat.Status `yaml:"stat"`
	}
)

// Validate validates poolSpec.
func (s PoolSpec) Validate() error {
	if s.ServiceName == "" && len(s.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range s.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(s.Servers) {
		return fmt.Errorf("not all servers have weight(%d/%d)",
			serversGotWeight, len(s.Servers))
	}

	if s.ServiceName == "" {
		servers := newStaticServers(s.Servers, s.ServersTags, s.LoadBalance)
		if servers.len() == 0 {
			return fmt.Errorf("serversTags picks none of servers")
		}
	}

	return nil
}

func newPool(super *supervisor.Supervisor, spec *PoolSpec, tagPrefix string) *pool {

	return &pool{
		spec:       spec,
		tagPrefix:  tagPrefix,
		servers:    newServers(super, spec),
		layer4stat: layer4stat.New(),
	}
}

func (p *pool) status() *PoolStatus {
	s := &PoolStatus{Stat: p.layer4stat.Status()}
	return s
}

func (p *pool) handle(ctx context.Layer4Context) string {

	server, err := p.servers.next(ctx)
	if err != nil {
		return resultInternalError
	}

	upstreamConn, err := net.DialTimeout("tcp", server.HostPort, 1000*time.Millisecond)
	if err != nil {
		logger.Errorf("dial tcp for addr: % failed, err: %v", server.HostPort, err)
	}
	_ = upstreamConn.Close()

	// TODO do layer4 proxy

	return ""
}

func (p *pool) close() {
	p.servers.close()
}
