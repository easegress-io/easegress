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
	stdcontext "context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/limitlistener"
)

type ListenerState int

type listener struct {
	name      string
	protocol  string        // enum:udp/tcp
	localAddr string        // listen addr
	state     ListenerState // listener state

	mutex    *sync.Mutex
	stopChan chan struct{}
	maxConns uint32 // maxConn for tcp listener

	udpListener net.PacketConn                                                                                         // udp listener
	tcpListener *limitlistener.LimitListener                                                                           // tcp listener with accept limit
	onTcpAccept func(conn net.Conn, listenerStop chan struct{})                                                        // tcp accept handle
	onUdpAccept func(downstreamAddr net.Addr, conn net.Conn, listenerStop chan struct{}, packet iobufferpool.IoBuffer) // udp accept handle
}

func newListener(spec *Spec, onTcpAccept func(conn net.Conn, listenerStop chan struct{}),
	onUdpAccept func(cliAddr net.Addr, conn net.Conn, listenerStop chan struct{}, packet iobufferpool.IoBuffer)) *listener {
	listen := &listener{
		name:      spec.Name,
		protocol:  spec.Protocol,
		localAddr: fmt.Sprintf(":%d", spec.Port),

		mutex:    &sync.Mutex{},
		stopChan: make(chan struct{}),
	}

	if listen.protocol == "tcp" {
		listen.maxConns = spec.MaxConnections
		listen.onTcpAccept = onTcpAccept
	} else {
		listen.onUdpAccept = onUdpAccept
	}
	return listen
}

func (l *listener) listen() error {
	switch l.protocol {
	case "udp":
		c := net.ListenConfig{}
		if ul, err := c.ListenPacket(stdcontext.Background(), l.protocol, l.localAddr); err != nil {
			return err
		} else {
			l.udpListener = ul
		}
	case "tcp":
		if tl, err := net.Listen(l.protocol, l.localAddr); err != nil {
			return err
		} else {
			// wrap tcp listener with accept limit
			l.tcpListener = limitlistener.NewLimitListener(tl, l.maxConns)
		}
	default:
		return errors.New("invalid protocol for layer4 server listener")
	}
	return nil
}

func (l *listener) startEventLoop() {
	switch l.protocol {
	case "udp":
		l.readMsgEventLoop()
	case "tcp":
		l.acceptEventLoop()
	}
}

func (l *listener) readMsgEventLoop() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("failed to read udp msg for %s\n, stack trace: \n", l.localAddr, debug.Stack())
				l.readMsgEventLoop()
			}
		}()

		l.readMsgLoop()
	}()
}

func (l *listener) readMsgLoop() {
	conn := l.udpListener.(*net.UDPConn)
	buf := iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
	defer func(buf iobufferpool.IoBuffer) {
		_ = iobufferpool.PutIoBuffer(buf)
	}(buf)

	for {
		buf.Reset()
		n, rAddr, err := conn.ReadFromUDP(buf.Bytes()[:buf.Cap()])
		_ = buf.Grow(n)

		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Infof("udp listener %s stop receiving packet by deadline", l.localAddr)
				return
			}
			if ope, ok := err.(*net.OpError); ok {
				if !(ope.Timeout() && ope.Temporary()) {
					logger.Errorf("udp listener %s occurs non-recoverable error, stop listening and receiving", l.localAddr)
					return
				}
			}
			logger.Errorf("udp listener %s receiving packet occur error: %+v", l.localAddr, err)
			continue
		}
		l.onUdpAccept(rAddr, conn, l.stopChan, buf.Clone())
	}
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
						logger.Errorf("tcp listener(%s) stop accept connection due to listener closed",
							l.localAddr)
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
			go l.onTcpAccept(tconn, l.stopChan)
		}
	}
}

func (l *listener) setMaxConnection(maxConn uint32) {
	l.tcpListener.SetMaxConnection(maxConn)
}

func (l *listener) close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	var err error
	switch l.protocol {
	case "tcp":
		if l.tcpListener != nil {
			err = l.tcpListener.Close()
		}
	case "udp":
		if l.udpListener != nil {
			err = l.udpListener.Close()
		}
	}
	close(l.stopChan)
	return err
}
