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
	"fmt"
	"net"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/connection"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
)

const (
	checkFailedTimeout = 10 * time.Second

	stateNil     stateType = "nil"
	stateFailed  stateType = "failed"
	stateRunning stateType = "running"
	stateClosed  stateType = "closed"
)

var (
	errNil = fmt.Errorf("")
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
		muxMapper     protocol.MuxMapper
	}
	eventClose struct{ done chan struct{} }

	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec
		mux       *mux
		pool      *pool     // backend servers
		listener  *listener // layer4 server
		startNum  uint64
		eventChan chan interface{} // receive traffic controller event

		state atomic.Value // runtime running state
		err   atomic.Value // runtime running error
	}
)

func newRuntime(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) *runtime {
	r := &runtime{
		superSpec: superSpec,
		eventChan: make(chan interface{}, 10),
	}

	r.mux = newMux(muxMapper)
	r.setState(stateNil)
	r.setError(errNil)

	go r.fsm()
	go r.checkFailed()
	return r
}

// Close notify runtime close
func (r *runtime) Close() {
	done := make(chan struct{})
	r.eventChan <- &eventClose{done: done}
	<-done
}

// FSM is the finite-state-machine for the runtime.
func (r *runtime) fsm() {
	for e := range r.eventChan {
		switch e := e.(type) {
		case *eventCheckFailed:
			r.handleEventCheckFailed(e)
		case *eventServeFailed:
			r.handleEventServeFailed(e)
		case *eventReload:
			r.handleEventReload(e)
		case *eventClose:
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

func (r *runtime) reload(nextSuperSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	r.superSpec = nextSuperSpec
	r.mux.reloadRules(nextSuperSpec, muxMapper)

	nextSpec := nextSuperSpec.ObjectSpec().(*Spec)
	r.pool = newPool(nextSuperSpec.Super(), nextSpec.Pool, "")

	// r.listener does not create just after the process started and the config load for the first time.
	if nextSpec != nil && r.listener != nil {
		r.listener.setMaxConnection(nextSpec.MaxConnections)
	}

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

func (r *runtime) setState(state stateType) {
	r.state.Store(state)
}

func (r *runtime) getState() stateType {
	return r.state.Load().(stateType)
}

func (r *runtime) setError(err error) {
	if err == nil {
		r.err.Store(errNil)
	} else {
		// NOTE: For type safe.
		r.err.Store(fmt.Errorf("%v", err))
	}
}

func (r *runtime) getError() error {
	err := r.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

func (r *runtime) needRestartServer(nextSpec *Spec) bool {
	x := *r.spec
	y := *nextSpec

	// The change of options below need not restart the layer4 server.
	x.KeepAlive, y.KeepAlive = true, true
	x.MaxConnections, y.MaxConnections = 0, 0
	x.ConnectTimeout, y.ProxyTimeout = 0, 0
	x.ProxyTimeout, y.ProxyTimeout = 0, 0

	x.Pool, y.Pool = nil, nil
	x.IPFilter, y.IPFilter = nil, nil

	// The update of rules need not to shutdown server.
	return !reflect.DeepEqual(x, y)
}

func (r *runtime) startServer() {
	l := newListener(r.spec, r.onTcpAccept(), r.onUdpAccept())

	r.listener = l
	r.startNum++
	r.setState(stateRunning)
	r.setError(nil)

	if err := l.listen(); err != nil {
		r.setState(stateFailed)
		r.setError(err)
		logger.Errorf("listen for %s %s failed, err: %+v", l.protocol, l.localAddr, err)

		_ = l.close()
		r.eventChan <- &eventServeFailed{
			err:      err,
			startNum: r.startNum,
		}
		return
	}

	go r.listener.startEventLoop()
}

func (r *runtime) closeServer() {
	if r.listener == nil {
		return
	}

	_ = r.listener.close()
	logger.Infof("listener for %s :%d closed", r.listener.protocol, r.listener.localAddr)
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

func (r *runtime) handleEventCheckFailed(e *eventCheckFailed) {
	if r.getState() == stateFailed {
		r.startServer()
	}
}

func (r *runtime) handleEventServeFailed(e *eventServeFailed) {
	if r.startNum > e.startNum {
		return
	}
	r.setState(stateFailed)
	r.setError(e.err)
}

func (r *runtime) handleEventReload(e *eventReload) {
	r.reload(e.nextSuperSpec, e.muxMapper)
}

func (r *runtime) handleEventClose(e *eventClose) {
	r.closeServer()
	r.mux.close()
	r.pool.close()
	close(e.done)
}

func (r *runtime) onTcpAccept() func(conn net.Conn, listenerStop chan struct{}) {

	return func(rawConn net.Conn, listenerStop chan struct{}) {
		downstream := rawConn.RemoteAddr().(*net.TCPAddr).IP.String()
		if r.mux.AllowIP(downstream) {
			_ = rawConn.Close()
			logger.Infof("close tcp connection from %s to %s which ip is not allowed",
				rawConn.RemoteAddr().String(), rawConn.LocalAddr().String())
			return
		}

		server, err := r.pool.servers.next(downstream)
		if err != nil {
			_ = rawConn.Close()
			logger.Errorf("close tcp connection due to no available upstream server, local addr: %s, err: %+v",
				rawConn.LocalAddr(), err)
			return
		}

		upstreamAddr, _ := net.ResolveTCPAddr("tcp", server.Addr)
		upstreamConn := connection.NewUpstreamConn(time.Duration(r.spec.ConnectTimeout)*time.Millisecond, upstreamAddr, listenerStop)
		if err := upstreamConn.Connect(); err != nil {
			logger.Errorf("close tcp connection due to upstream conn connect failed, local addr: %s, err: %+v",
				rawConn.LocalAddr().String(), err)
			_ = rawConn.Close()
		} else {
			downstreamConn := connection.NewDownstreamConn(rawConn, rawConn.RemoteAddr(), listenerStop)
			ctx := context.NewLayer4Context("tcp", rawConn.LocalAddr(), rawConn.RemoteAddr(), upstreamAddr)
			r.setOnReadHandler(downstreamConn, upstreamConn, ctx)
		}
	}
}

func (r *runtime) onUdpAccept() func(cliAddr net.Addr, rawConn net.Conn, listenerStop chan struct{}, packet iobufferpool.IoBuffer) {
	return func(cliAddr net.Addr, rawConn net.Conn, listenerStop chan struct{}, packet iobufferpool.IoBuffer) {
		downstream := cliAddr.(*net.UDPAddr).IP.String()
		if r.mux.AllowIP(downstream) {
			logger.Infof("discard udp packet from %s to %s which ip is not allowed", cliAddr.String(),
				rawConn.LocalAddr().String())
			return
		}

		localAddr := rawConn.LocalAddr()
		key := connection.GetProxyMapKey(localAddr.String(), cliAddr.String())
		if rawDownstreamConn, ok := connection.ProxyMap.Load(key); ok {
			downstreamConn := rawDownstreamConn.(*connection.Connection)
			downstreamConn.OnRead(packet)
			return
		}

		server, err := r.pool.servers.next(downstream)
		if err != nil {
			logger.Infof("discard udp packet from %s to %s due to can not find upstream server, err: %+v",
				cliAddr.String(), localAddr.String())
			return
		}

		upstreamAddr, _ := net.ResolveUDPAddr("udp", server.Addr)
		upstreamConn := connection.NewUpstreamConn(time.Duration(r.spec.ConnectTimeout)*time.Millisecond, upstreamAddr, listenerStop)
		if err := upstreamConn.Connect(); err != nil {
			logger.Errorf("discard udp packet due to upstream connect failed, local addr: %s, err: %+v", localAddr, err)
			return
		}

		downstreamConn := connection.NewDownstreamConn(rawConn, rawConn.RemoteAddr(), listenerStop)
		ctx := context.NewLayer4Context("udp", localAddr, upstreamAddr, upstreamAddr)
		connection.SetUDPProxyMap(connection.GetProxyMapKey(localAddr.String(), cliAddr.String()), &downstreamConn)
		r.setOnReadHandler(downstreamConn, upstreamConn, ctx)
		downstreamConn.OnRead(packet)
	}
}

func (r *runtime) setOnReadHandler(downstreamConn *connection.Connection, upstreamConn *connection.UpstreamConnection, ctx context.Layer4Context) {
	if handle, ok := r.mux.GetHandler(r.spec.Name); ok {
		downstreamConn.SetOnRead(func(readBuf iobufferpool.IoBuffer) {
			handle.Handle(ctx, readBuf, nil)
			if buf := ctx.GetDownstreamWriteBuffer(); buf != nil {
				_ = upstreamConn.Write(buf)
			}
		})
		upstreamConn.SetOnRead(func(readBuf iobufferpool.IoBuffer) {
			handle.Handle(ctx, readBuf, nil)
			if buf := ctx.GetUpstreamWriteBuffer(); buf != nil {
				_ = downstreamConn.Write(buf)
			}
		})
	} else {
		downstreamConn.SetOnRead(func(readBuf iobufferpool.IoBuffer) {
			if readBuf != nil && readBuf.Len() > 0 {
				_ = upstreamConn.Write(readBuf.Clone())
				readBuf.Drain(readBuf.Len())
			}
		})
		upstreamConn.SetOnRead(func(readBuf iobufferpool.IoBuffer) {
			if readBuf != nil && readBuf.Len() > 0 {
				_ = downstreamConn.Write(readBuf.Clone())
				readBuf.Drain(readBuf.Len())
			}
		})
	}
}
