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

package tcpproxy

import (
	"fmt"
	"net"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/layer4backend"
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
	}
	eventClose struct{ done chan struct{} }

	runtime struct {
		superSpec *supervisor.Spec
		spec      *Spec

		pool      *layer4backend.Pool       // backend servers pool
		ipFilters *ipfilter.Layer4IpFilters // ip filters
		listener  *listener                 // tcp listener

		startNum  uint64
		eventChan chan interface{} // receive traffic controller event

		state atomic.Value // runtime running state
		err   atomic.Value // runtime running error
	}
)

func newRuntime(superSpec *supervisor.Spec) *runtime {
	spec := superSpec.ObjectSpec().(*Spec)
	r := &runtime{
		superSpec: superSpec,

		pool:      layer4backend.NewPool(superSpec.Super(), spec.Pool, ""),
		ipFilters: ipfilter.NewLayer4IPFilters(spec.IPFilter),

		eventChan: make(chan interface{}, 10),
	}

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
			r.handleEventCheckFailed()
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

func (r *runtime) reload(nextSuperSpec *supervisor.Spec) {
	r.superSpec = nextSuperSpec
	nextSpec := nextSuperSpec.ObjectSpec().(*Spec)
	r.ipFilters.ReloadRules(nextSpec.IPFilter)
	r.pool.ReloadRules(nextSuperSpec.Super(), nextSpec.Pool, "")

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

	// The change of options below need not restart the tcp server.
	x.MaxConnections, y.MaxConnections = 0, 0
	x.ConnectTimeout, y.ConnectTimeout = 0, 0

	x.Pool, y.Pool = nil, nil
	x.IPFilter, y.IPFilter = nil, nil

	// The update of rules need not to shutdown server.
	return !reflect.DeepEqual(x, y)
}

func (r *runtime) startServer() {
	l := newListener(r.spec, r.onAccept())

	r.listener = l
	r.startNum++
	r.setState(stateRunning)
	r.setError(nil)

	if err := l.listen(); err != nil {
		r.setState(stateFailed)
		r.setError(err)
		logger.Errorf("tcp listener for %s failed, err: %+v", l.localAddr, err)

		_ = l.close()
		r.eventChan <- &eventServeFailed{
			err:      err,
			startNum: r.startNum,
		}
		return
	}

	go r.listener.acceptEventLoop()
}

func (r *runtime) closeServer() {
	if r.listener == nil {
		return
	}

	_ = r.listener.close()
	logger.Infof("listener for %s(%s) closed", r.listener.name, r.listener.localAddr)
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
	r.setError(e.err)
}

func (r *runtime) handleEventReload(e *eventReload) {
	r.reload(e.nextSuperSpec)
}

func (r *runtime) handleEventClose(e *eventClose) {
	r.closeServer()
	r.pool.Close()
	close(e.done)
}

func (r *runtime) onAccept() func(conn net.Conn, listenerStop chan struct{}) {

	return func(rawConn net.Conn, listenerStop chan struct{}) {
		clientIP := rawConn.RemoteAddr().(*net.TCPAddr).IP.String()
		if r.ipFilters != nil && !r.ipFilters.AllowIP(clientIP) {
			_ = rawConn.Close()
			logger.Infof("close tcp connection from %s to %s which ip is not allowed",
				rawConn.RemoteAddr().String(), rawConn.LocalAddr().String())
			return
		}

		server, err := r.pool.Next(clientIP)
		if err != nil {
			_ = rawConn.Close()
			logger.Errorf("close tcp connection due to no available server, local addr: %s, err: %+v",
				rawConn.LocalAddr(), err)
			return
		}

		serverAddr, _ := net.ResolveTCPAddr("tcp", server.Addr)
		serverConn := NewServerConn(r.spec.ConnectTimeout, serverAddr, listenerStop)
		if !serverConn.Connect() {
			_ = rawConn.Close()
			return
		}

		clientConn := NewClientConn(rawConn, listenerStop)
		r.setCallbacks(clientConn, serverConn)
		clientConn.Start() // server conn start read/write loop when connect is called
	}
}

func (r *runtime) setCallbacks(clientConn *Connection, serverConn *ServerConnection) {
	clientConn.SetOnRead(func(readBuf *iobufferpool.StreamBuffer) {
		if readBuf != nil && readBuf.Len() > 0 {
			_ = serverConn.Write(readBuf)
		}
	})
	serverConn.SetOnRead(func(readBuf *iobufferpool.StreamBuffer) {
		if readBuf != nil && readBuf.Len() > 0 {
			_ = clientConn.Write(readBuf)
		}
	})

	clientConn.SetOnClose(func(event ConnectionEvent) {
		if event == RemoteClose {
			_ = serverConn.Close(FlushWrite, LocalClose)
		} else {
			_ = serverConn.Close(NoFlush, LocalClose)
		}
	})
	serverConn.SetOnClose(func(event ConnectionEvent) {
		if event == RemoteClose {
			_ = clientConn.Close(FlushWrite, LocalClose)
		} else {
			_ = clientConn.Close(NoFlush, LocalClose)
		}
	})
}
