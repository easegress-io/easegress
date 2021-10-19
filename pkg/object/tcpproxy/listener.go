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
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/limitlistener"
)

type ListenerState int

type listener struct {
	name      string
	localAddr string        // listen addr
	state     ListenerState // listener state

	mutex    *sync.Mutex
	stopChan chan struct{}
	maxConns uint32 // maxConn for tcp listener

	tcpListener *limitlistener.LimitListener                    // tcp listener with accept limit
	onAccept    func(conn net.Conn, listenerStop chan struct{}) // tcp accept handle
}

func newListener(spec *Spec, onAccept func(conn net.Conn, listenerStop chan struct{})) *listener {
	listen := &listener{
		name:      spec.Name,
		localAddr: fmt.Sprintf(":%d", spec.Port),

		onAccept: onAccept,
		maxConns: spec.MaxConnections,

		mutex:    &sync.Mutex{},
		stopChan: make(chan struct{}),
	}
	return listen
}

func (l *listener) listen() error {
	if tl, err := net.Listen("tcp", l.localAddr); err != nil {
		return err
	} else {
		// wrap tcp listener with accept limit
		l.tcpListener = limitlistener.NewLimitListener(tl, l.maxConns)
	}
	return nil
}

func (l *listener) acceptEventLoop() {

	for {
		if tconn, err := l.tcpListener.Accept(); err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Infof("tcp listener(%s) stop accept connection due to timeout, err: %s",
					l.localAddr, nerr)
				return
			}

			if ope, ok := err.(*net.OpError); ok {
				// not timeout error and not temporary, which means the error is non-recoverable
				if !(ope.Timeout() && ope.Temporary()) {
					// accept error raised by sockets closing
					if ope.Op == "accept" {
						logger.Debugf("tcp listener(%s) stop accept connection due to listener closed", l.localAddr)
					} else {
						logger.Errorf("tcp listener(%s) stop accept connection due to non-recoverable error: %s",
							l.localAddr, err.Error())
					}
					return
				}
			} else {
				logger.Errorf("tcp listener(%s) stop accept connection with unknown error: %s.",
					l.localAddr, err.Error())
			}
		} else {
			go l.onAccept(tconn, l.stopChan)
		}
	}
}

func (l *listener) setMaxConnection(maxConn uint32) {
	l.tcpListener.SetMaxConnection(maxConn)
}

func (l *listener) close() (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.tcpListener != nil {
		err = l.tcpListener.Close()
	}
	close(l.stopChan)
	return err
}
