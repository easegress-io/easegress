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

package layer4proxy

import (
	"fmt"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/layer4filter"
	"github.com/megaease/easegress/pkg/util/layer4stat"
	"net"
)

type (
	pool struct {
		spec *PoolSpec

		tagPrefix string
		filter    *layer4filter.Layer4filter

		servers    *servers
		layer4Stat *layer4stat.Layer4Stat
	}

	// PoolSpec describes a pool of servers.
	PoolSpec struct {
		SpanName        string             `yaml:"spanName" jsonschema:"omitempty"`
		ServersTags     []string           `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		Filter          *layer4filter.Spec `yaml:"filter" jsonschema:"omitempty"`
		Servers         []*Server          `yaml:"servers" jsonschema:"omitempty"`
		ServiceRegistry string             `yaml:"serviceRegistry" jsonschema:"omitempty"`
		ServiceName     string             `yaml:"serviceName" jsonschema:"omitempty"`
		LoadBalance     *LoadBalance       `yaml:"loadBalance" jsonschema:"required"`
	}

	// PoolStatus is the status of Pool.
	PoolStatus struct {
		Stat *layer4stat.Status `yaml:"stat"`
	}

	UpStreamConn struct {
		conn            net.Conn
		done            chan struct{}
		writeBufferChan chan iobufferpool.IoBuffer
	}
)

func NewUpStreamConn(conn net.Conn) *UpStreamConn {
	return &UpStreamConn{
		conn:            conn,
		writeBufferChan: make(chan iobufferpool.IoBuffer, 8),
	}
}

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
		spec:      spec,
		tagPrefix: tagPrefix,

		servers:    newServers(super, spec),
		layer4Stat: layer4stat.New(),
	}
}

func (p *pool) status() *PoolStatus {
	s := &PoolStatus{Stat: p.layer4Stat.Status()}
	return s
}

func (u *UpStreamConn) Write(source iobufferpool.IoBuffer) {
	buf := source.Clone()
	source.Drain(buf.Len())
	u.writeBufferChan <- buf
}

func (u *UpStreamConn) WriteLoop() {
	for {
		select {
		case buf, ok := <-u.writeBufferChan:
			if !ok {
				return
			}

			iobuf := buf.(iobufferpool.IoBuffer)
			for {
				n, err := u.conn.Write(iobuf.Bytes())
				if n == 0 || err != nil {
					return
				}
				iobuf.Drain(n)
			}
		case <-u.done:
			return
		}
	}
}

func (p *pool) handle(ctx context.Layer4Context) string {

	conn := ctx.UpStreamConn()
	if conn == nil {
		server, err := p.servers.next(ctx)
		if err != nil {
			return resultInternalError
		}

		switch ctx.Protocol() {
		case "tcp":
			if tconn, dialErr := net.Dial("tcp", server.Addr); dialErr != nil {
				logger.Errorf("dial tcp to %s failed, err: %s", server.Addr, dialErr.Error())
				return resultServerError
			} else {
				upstreamConn := NewUpStreamConn(tconn)
				ctx.SetUpStreamConn(upstreamConn)
				go upstreamConn.WriteLoop()

				go func() {
					// TODO do upstream connection read
				}()
			}
		case "udp":

		}
	}

	return ""
}

func (p *pool) close() {
	p.servers.close()
}
