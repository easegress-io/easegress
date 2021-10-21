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
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
)

const (
	checkFailedTimeout = 10 * time.Second

	stateNil     stateType = "nil"
	stateFailed  stateType = "failed"
	stateRunning stateType = "running"
	stateClosed  stateType = "closed"
)

type (
	stateType string

	eventCheckFailed struct{}
	eventServeFailed struct {
		startNum uint64
		err      error
	}

	eventReload struct {
		nextSuperSpec *supervisor.Spec
	}
	eventClose struct{ done chan struct{} }

	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec

		startNum   uint64
		pool       *pool        // backend servers pool
		serverConn *net.UDPConn // listener
		sessions   map[string]*session

		state     atomic.Value     // runtime running state
		eventChan chan interface{} // receive event
		ipFilters *ipFilters

		mu sync.Mutex
	}
)

func newRuntime(superSpec *supervisor.Spec) *runtime {
	spec := superSpec.ObjectSpec().(*Spec)
	r := &runtime{
		superSpec: superSpec,

		pool:      newPool(superSpec.Super(), spec.Pool, ""),
		ipFilters: newIPFilters(spec.IPFilter),

		eventChan: make(chan interface{}, 10),
		sessions:  make(map[string]*session),
	}

	r.setState(stateNil)

	go r.fsm()
	go r.checkFailed()
	return r
}

// FSM is the finite-state-machine for the runtime.
func (r *runtime) fsm() {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case e := <-r.eventChan:
			switch e := e.(type) {
			case *eventCheckFailed:
				r.handleEventCheckFailed()
			case *eventServeFailed:
				r.handleEventServeFailed(e)
			case *eventReload:
				r.handleEventReload(e)
			case *eventClose:
				ticker.Stop()
				r.handleEventClose(e)
				// NOTE: We don't close hs.eventChan,
				// in case of panic of any other goroutines
				// to send event to it later.
				return
			default:
				logger.Errorf("BUG: unknown event: %T\n", e)
			}
		}
	}
}

func (r *runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *runtime) getState() stateType {
	return r.state.Load().(stateType)
}

// Close notify runtime close
func (r *runtime) Close() {
	done := make(chan struct{})
	r.eventChan <- &eventClose{done: done}
	<-done
}

func (r *runtime) checkFailed() {
	ticker := time.NewTicker(checkFailedTimeout)
	for range ticker.C {
		state := r.getState()
		if state == stateFailed {
			r.eventChan <- &eventCheckFailed{}
		} else if state == stateClosed {
			ticker.Stop()
			return
		}
	}
}

func (r *runtime) handleEventCheckFailed() {
	if r.getState() == stateFailed {
		r.startServer()
	}
}

func (r *runtime) handleEventServeFailed(e *eventServeFailed) {
	if r.startNum > e.startNum {
		return
	}
	r.setState(stateFailed)
}

func (r *runtime) handleEventReload(e *eventReload) {
	r.reload(e.nextSuperSpec)
}

func (r *runtime) handleEventClose(e *eventClose) {
	r.closeServer()
	r.pool.close()
	close(e.done)
}

func (r *runtime) reload(nextSuperSpec *supervisor.Spec) {
	r.superSpec = nextSuperSpec
	nextSpec := nextSuperSpec.ObjectSpec().(*Spec)

	r.ipFilters.reloadRules(nextSpec.IPFilter)
	r.pool.reloadRules(nextSuperSpec.Super(), nextSpec.Pool, "")

	// NOTE: Due to the mechanism of supervisor,
	// nextSpec must not be nil, just defensive programming here.
	switch {
	case r.spec == nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		// Nothing to do.
	case r.spec == nil && nextSpec != nil:
		r.spec = nextSpec
		r.startServer()
	case r.spec != nil && nextSpec == nil:
		logger.Errorf("BUG: nextSpec is nil")
		r.spec = nil
		r.closeServer()
	case r.spec != nil && nextSpec != nil:
		if r.needRestartServer(nextSpec) {
			r.spec = nextSpec
			r.closeServer()
			r.startServer()
		} else {
			r.spec = nextSpec
		}
	}
}

func (r *runtime) startServer() {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.spec.Port))
	if err != nil {
		r.setState(stateFailed)
		logger.Errorf("parse udp listen addr(%s) failed, err: %+v", r.spec.Port, err)
		return
	}

	r.serverConn, err = net.ListenUDP("udp", listenAddr)
	if err != nil {
		r.setState(stateFailed)
		logger.Errorf("create udp listener(%s) failed, err: %+v", r.spec.Port, err)
		return
	}
	r.setState(stateRunning)

	var cp *connPool
	if r.spec.HasResponse {
		cp = newConnPool()
	}

	go func() {
		defer cp.close()

		buf := iobufferpool.GetIoBuffer(iobufferpool.UDPPacketMaxSize)
		for {
			buf.Reset()
			n, downstreamAddr, err := r.serverConn.ReadFromUDP(buf.Bytes()[:buf.Cap()])
			_ = buf.Grow(n)

			if err != nil {
				if r.getState() != stateRunning {
					return
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

			if r.ipFilters != nil {
				if !r.ipFilters.AllowIP(downstreamAddr.IP.String()) {
					logger.Debugf("discard udp packet from %s send to udp server(:%d)", downstreamAddr.IP.String(), r.spec.Port)
					continue
				}
			}

			if !r.spec.HasResponse {
				if err := r.sendOneShot(cp, downstreamAddr, &buf); err != nil {
					logger.Errorf("%s", err.Error())
				}
				continue
			}

			data := buf.Clone()
			r.proxy(downstreamAddr, &data)
		}
	}()
}

func (r *runtime) getUpstreamConn(pool *connPool, downstreamAddr *net.UDPAddr) (net.Conn, string, error) {
	server, err := r.pool.next(downstreamAddr.IP.String())
	if err != nil {
		return nil, "", fmt.Errorf("can not get upstream addr for udp connection(:%d)", r.spec.Port)
	}

	var upstreamConn net.Conn
	if pool != nil {
		upstreamConn = pool.get(server.Addr)
		if upstreamConn != nil {
			return upstreamConn, server.Addr, nil
		}
	}

	addr, err := net.ResolveUDPAddr("udp", server.Addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("parse upstream addr(%s) to udp addr failed, err: %+v", server.Addr, err)
	}

	upstreamConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, server.Addr, fmt.Errorf("dial to upstream addr(%s) failed, err: %+v", server.Addr, err)
	}
	if pool != nil {
		pool.put(server.Addr, upstreamConn)
	}
	return upstreamConn, server.Addr, nil
}

func (r *runtime) sendOneShot(pool *connPool, downstreamAddr *net.UDPAddr, buf *iobufferpool.IoBuffer) error {
	upstreamConn, upstreamAddr, err := r.getUpstreamConn(pool, downstreamAddr)
	if err != nil {
		return err
	}

	n, err := upstreamConn.Write((*buf).Bytes())
	if err != nil {
		return fmt.Errorf("sned data to %s failed, err: %+v", upstreamAddr, err)
	}

	if n != (*buf).Len() {
		return fmt.Errorf("failed to send full packet to %s, read %d but send %d", upstreamAddr, (*buf).Len(), n)
	}
	return nil
}

func (r *runtime) getSession(downstreamAddr *net.UDPAddr) (*session, error) {
	key := downstreamAddr.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[key]
	if ok && !s.IsClosed() {
		return s, nil
	}

	if ok {
		go func() { s.Close() }()
	}

	upstreamConn, upstreamAddr, err := r.getUpstreamConn(nil, downstreamAddr)
	if err != nil {
		return nil, err
	}

	s = newSession(downstreamAddr, upstreamAddr, upstreamConn,
		time.Duration(r.spec.UpstreamIdleTimeout)*time.Millisecond, time.Duration(r.spec.DownstreamIdleTimeout)*time.Millisecond)
	s.ListenResponse(r.serverConn)

	r.sessions[key] = s
	return s, nil
}

func (r *runtime) proxy(downstreamAddr *net.UDPAddr, buf *iobufferpool.IoBuffer) {
	s, err := r.getSession(downstreamAddr)
	if err != nil {
		logger.Errorf("%s", err.Error())
		return
	}

	err = s.Write(buf)
	if err != nil {
		logger.Errorf("write data to udp session(%s) failed, err: %v", downstreamAddr.IP.String(), err)
	}
}

func (r *runtime) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, s := range r.sessions {
		if s.IsClosed() {
			delete(r.sessions, k)
		}
	}
}

func (r *runtime) closeServer() {
	r.setState(stateClosed)
	_ = r.serverConn.Close()
	r.mu.Lock()
	for k, s := range r.sessions {
		delete(r.sessions, k)
		s.Close()
	}
	r.mu.Unlock()
}

func (r *runtime) needRestartServer(nextSpec *Spec) bool {
	x := *r.spec
	y := *nextSpec

	x.Pool, y.Pool = nil, nil
	x.IPFilter, y.IPFilter = nil, nil

	return !reflect.DeepEqual(x, y)
}
