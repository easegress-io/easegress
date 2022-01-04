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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/layer4backend"
)

type (
	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec

		pool       *layer4backend.Pool // backend servers pool
		serverConn *net.UDPConn        // listener
		sessions   map[string]*session

		ipFilters *ipfilter.Layer4IpFilters

		mu   sync.Mutex
		done chan struct{}
	}
)

func newRuntime(superSpec *supervisor.Spec) *runtime {
	spec := superSpec.ObjectSpec().(*Spec)
	r := &runtime{
		superSpec: superSpec,
		spec:      spec,

		pool:      layer4backend.NewPool(superSpec.Super(), spec.Pool, ""),
		ipFilters: ipfilter.NewLayer4IPFilters(spec.IPFilter),

		done:     make(chan struct{}),
		sessions: make(map[string]*session),
	}

	r.startServer()
	return r
}

// close notify runtime close
func (r *runtime) close() {
	close(r.done)
	_ = r.serverConn.Close()
	r.pool.Close()
}

func (r *runtime) startServer() {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.spec.Port))
	if err != nil {
		logger.Errorf("parse udp listen addr(%s) failed, err: %+v", r.spec.Port, err)
		return
	}

	r.serverConn, err = net.ListenUDP("udp", listenAddr)
	if err != nil {
		logger.Errorf("create udp listener(%s) failed, err: %+v", r.spec.Port, err)
		return
	}

	var cp *connPool
	if !r.spec.HasResponse {
		// if client udp request doesn't have response, use connection pool to save server connections pool
		cp = newConnPool()
	}

	go func() {
		defer cp.close()

		buf := make([]byte, iobufferpool.UDPPacketMaxSize)
		for {
			n, clientAddr, err := r.serverConn.ReadFromUDP(buf[:])

			if err != nil {
				select {
				case <-r.done:
					return // detect weather udp server is closed
				default:
				}

				if ope, ok := err.(*net.OpError); ok {
					// not timeout error and not temporary, which means the error is non-recoverable
					if !(ope.Timeout() && ope.Temporary()) {
						logger.Errorf("udp listener(%d) crashed due to non-recoverable error, err: %+v", r.spec.Port, err)
						return
					}
				}
				logger.Errorf("failed to read packet from udp connection(:%d), err: %+v", r.spec.Port, err)
				continue
			}

			if !r.ipFilters.AllowIP(clientAddr.IP.String()) {
				logger.Debugf("discard udp packet from %s send to udp server(:%d)", clientAddr.IP.String(), r.spec.Port)
				continue
			}

			if !r.spec.HasResponse {
				if err := r.sendOneShot(cp, clientAddr, buf[0:n]); err != nil {
					logger.Errorf("%s", err.Error())
				}
				continue
			}

			r.proxy(clientAddr, buf[0:n])
		}
	}()
}

func (r *runtime) getServerConn(pool *connPool, clientAddr *net.UDPAddr) (net.Conn, string, error) {
	server, err := r.pool.Next(clientAddr.IP.String())
	if err != nil {
		return nil, "", fmt.Errorf("can not get server addr for udp connection(:%d)", r.spec.Port)
	}

	var serverConn net.Conn
	if pool != nil {
		serverConn = pool.get(server.Addr)
		if serverConn != nil {
			return serverConn, server.Addr, nil
		}
	}

	addr, err := net.ResolveUDPAddr("udp", server.Addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("parse server addr(%s) to udp addr failed, err: %+v", server.Addr, err)
	}

	serverConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("dial to server addr(%s) failed, err: %+v", server.Addr, err)
	}

	if pool != nil {
		pool.put(server.Addr, serverConn)
	}
	return serverConn, server.Addr, nil
}

func (r *runtime) sendOneShot(pool *connPool, clientAddr *net.UDPAddr, buf []byte) error {
	serverConn, serverAddr, err := r.getServerConn(pool, clientAddr)
	if err != nil {
		return err
	}

	n, err := serverConn.Write(buf)
	if err != nil {
		return fmt.Errorf("sned data to %s failed, err: %+v", serverAddr, err)
	}

	if n != len(buf) {
		return fmt.Errorf("failed to send full packet to %s, read %d but send %d", serverAddr, len(buf), n)
	}
	return nil
}

func (r *runtime) getSession(clientAddr *net.UDPAddr) (*session, error) {
	key := clientAddr.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[key]
	if ok && !s.isClosed() {
		return s, nil
	}

	serverConn, serverAddr, err := r.getServerConn(nil, clientAddr)
	if err != nil {
		return nil, err
	}

	onClose := func() {
		r.mu.Lock()
		delete(r.sessions, key)
		r.mu.Unlock()
	}
	s = newSession(clientAddr, serverAddr, serverConn, r.done, onClose,
		time.Duration(r.spec.ServerIdleTimeout)*time.Millisecond,
		time.Duration(r.spec.ClientIdleTimeout)*time.Millisecond)
	s.ListenResponse(r.serverConn)

	r.sessions[key] = s
	return s, nil
}

func (r *runtime) proxy(clientAddr *net.UDPAddr, buf []byte) {
	s, err := r.getSession(clientAddr)
	if err != nil {
		logger.Errorf("%s", err.Error())
		return
	}

	dup := iobufferpool.UDPBufferPool.Get().([]byte)
	n := copy(dup, buf)
	err = s.Write(&iobufferpool.Packet{Payload: dup[:n], Len: n})
	if err != nil {
		logger.Errorf("write data to udp session(%s) failed, err: %v", clientAddr.IP.String(), err)
	}
}
