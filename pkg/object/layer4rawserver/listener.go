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
	stdcontext "context"
	"fmt"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/limitlistener"
	"net"
	"runtime/debug"
	"sync"
)

type ListenerState int

// listener state
// ListenerInited means listener is inited, an inited listener can be started or stopped
// ListenerRunning means listener is running, start a running listener will be ignored.
// ListenerStopped means listener is stopped, start a stopped listener without restart flag will be ignored.
const (
	ListenerInited ListenerState = iota
	ListenerRunning
	ListenerStopped
)

type listener struct {
	m           *mux
	name        string
	udpListener net.PacketConn               // udp connection listener
	tcpListener *limitlistener.LimitListener // tcp connection listener with connection limit

	state          ListenerState
	listenAddr     string
	protocol       string // enum:udp/tcp
	keepalive      bool
	reuseport      bool
	maxConnections uint32

	mutex    *sync.Mutex
	stopChan chan struct{} // connection listen to this stopChan

	onTcpAccept func(conn net.Conn, listenerStopChan chan struct{})
	onUdpAccept func(clientAddr net.Addr, buffer iobufferpool.IoBuffer)
}

func newListener(spec *Spec, onAccept func(conn net.Conn, listenerStopChan chan struct{})) *listener {
	listen := &listener{
		state:          ListenerInited,
		listenAddr:     fmt.Sprintf(":%d", spec.Port),
		protocol:       spec.Protocol,
		keepalive:      spec.KeepAlive,
		maxConnections: spec.MaxConnections,
		onTcpAccept:    onAccept,

		mutex: &sync.Mutex{},
	}
	return listen
}

func (l *listener) start() {
	ignored := func() bool {
		l.mutex.Lock()
		defer l.mutex.Unlock()

		switch l.state {
		case ListenerRunning:
			logger.Debugf("listener %s %s is already running", l.protocol, l.listenAddr)
			return true
		case ListenerStopped:
			logger.Debugf("listener %s %s restart", l.protocol, l.listenAddr)
			if err := l.listen(); err != nil {
				logger.Errorf("listener %s %s restart failed, err: %+v", l.protocol, l.listenAddr, err)
				return true
			}
		default:
			if l.udpListener == nil && l.tcpListener == nil {
				if err := l.listen(); err != nil {
					logger.Errorf("listener %s %s start failed, err: %+v", l.protocol, l.listenAddr, err)
				}
			}
		}
		l.state = ListenerRunning
		return false
	}()

	if ignored {
		return
	}

	switch l.protocol {
	case "udp":
		l.readMsgEventLoop()
	case "tcp":
		l.acceptEventLoop()
	}
}

func (l *listener) listen() error {
	switch l.protocol {
	case "udp":
		c := net.ListenConfig{}
		if ul, err := c.ListenPacket(stdcontext.Background(), l.protocol, l.listenAddr); err != nil {
			return err
		} else {
			l.udpListener = ul
		}
	case "tcp":
		if tl, err := net.Listen(l.protocol, l.listenAddr); err != nil {
			return err
		} else {
			l.tcpListener = limitlistener.NewLimitListener(tl, l.maxConnections)
		}
	}
	return nil
}

func (l *listener) acceptEventLoop() {

	for {
		if tconn, err := l.tcpListener.Accept(); err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Infof("tcp listener(%s) stop accept connection due to deadline, err: %s",
					l.listenAddr, nerr)
				return
			}

			if ope, ok := err.(*net.OpError); ok {
				// not timeout error and not temporary, which means the error is non-recoverable
				if !(ope.Timeout() && ope.Temporary()) {
					// accept error raised by sockets closing
					if ope.Op == "accept" {
						logger.Errorf("tcp listener(%s) stop accept connection due to listener closed",
							l.listenAddr)
					} else {
						logger.Errorf("tcp listener(%s) stop accept connection due to non-recoverable error: %s",
							l.listenAddr, err.Error())
					}
					return
				}
			} else {
				logger.Errorf("tcp listener(%s) stop accept connection with unknown error: %s.",
					l.listenAddr, err.Error())
			}
		} else {
			host, _, splitErr := net.SplitHostPort(tconn.RemoteAddr().String())
			if splitErr != nil || !l.m.AllowIP(host) {
				logger.Debugf("reject remote connection from: %s", tconn.RemoteAddr().String())
				_ = tconn.Close()
				continue
			}
			go l.onTcpAccept(tconn, l.stopChan)
		}
	}
}

func (l *listener) setMaxConnection(maxConn uint32) {
	l.tcpListener.SetMaxConnection(maxConn)
}

func (l *listener) readMsgEventLoop() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("failed to read udp msg for %s\n, stack trace: \n", l.listenAddr, debug.Stack())
				l.readMsgEventLoop()
			}
		}()

		l.readMsgLoop()
	}()
}

func (l *listener) readMsgLoop() {
	conn := l.udpListener.(*net.UDPConn)
	buf := iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
	defer iobufferpool.PutIoBuffer(buf)

	for {
		buf.Reset()
		n, rAddr, err := conn.ReadFromUDP(buf.Bytes()[:buf.Cap()])
		_ = buf.Grow(n)

		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Infof("udp listener %s stop receiving packet by deadline", l.listenAddr)
				return
			}
			if ope, ok := err.(*net.OpError); ok {
				if !(ope.Timeout() && ope.Temporary()) {
					logger.Errorf("udp listener %s occurs non-recoverable error, stop listening and receiving", l.listenAddr)
					return
				}
			}
			logger.Errorf("udp listener %s receiving packet occur error: %+v", l.listenAddr, err)
			continue
		}

		l.onUdpAccept(rAddr, buf)
	}
}

func (l *listener) close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.tcpListener != nil {
		return l.tcpListener.Close()
	}
	if l.udpListener != nil {
		return l.udpListener.Close()
	}
	close(l.stopChan) // TODO listener关闭时，需要关闭已建立的连接吗
	return nil
}
