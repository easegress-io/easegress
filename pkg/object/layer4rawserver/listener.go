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
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"time"

	context2 "github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/connectionwrapper"
	"github.com/megaease/easegress/pkg/util/limitlistener"
)

type listener struct {
	m             *Mux
	packetConn    net.PacketConn               // udp connection
	limitListener *limitlistener.LimitListener // tcp connection listener with connection limit

	state          stateType
	listenAddr     string
	protocol       string // enum:udp/tcp
	keepalive      bool
	reuseport      bool
	maxConnections uint32

	mutex *sync.Mutex
}

func NewListener(spec *Spec, m *Mux) *listener {
	listen := &listener{
		m:              m,
		protocol:       spec.Protocol,
		keepalive:      spec.KeepAlive,
		reuseport:      spec.Reuseport,
		maxConnections: spec.MaxConnections,
		mutex:          &sync.Mutex{},
	}
	if spec.LocalAddr == "" {
		listen.listenAddr = fmt.Sprintf(":%d", spec.Port)
	} else {
		listen.listenAddr = fmt.Sprintf("%s:%d", spec.LocalAddr, spec.Port)
	}
	return listen
}

func (l *listener) setMaxConnection(maxConn uint32) {
	l.limitListener.SetMaxConnection(maxConn)
}

func (l *listener) start() {

}

func (l *listener) listen() error {
	switch l.protocol {
	case "udp":
		c := net.ListenConfig{}
		if ul, err := c.ListenPacket(context.Background(), l.protocol, l.listenAddr); err != nil {
			return err
		} else {
			l.packetConn = ul
		}
	case "tcp":
		if tl, err := net.Listen(l.protocol, l.listenAddr); err != nil {
			return err
		} else {
			l.limitListener = limitlistener.NewLimitListener(tl, l.maxConnections)
		}
	}
	return nil
}

func (l *listener) accept(ctx context2.Layer4Context) error {
	rl, err := l.limitListener.Accept()
	if err != nil {
		return err
	}

	go func(ctx context2.Layer4Context) {
		if r := recover(); r != nil {
			logger.Errorf("failed tp accept conn for %s %s\n, stack trace: \n",
				l.protocol, l.listenAddr, debug.Stack())
		}

		ctx.SetRemoteAddr(rl.RemoteAddr()) // fix it
	}(ctx)
	return nil
}

func (l *listener) readUpdPacket(ctx context2.Layer4Context) {
	go func(ctx context2.Layer4Context) {
		if r := recover(); r != nil {
			logger.Errorf("failed tp accept conn for %s %s\n, stack trace: \n",
				l.protocol, l.listenAddr, debug.Stack())
		}

	}(ctx)
}

func (l *listener) acceptEventLoop() {

	for {
		if tconn, err := l.limitListener.Accept(); err != nil {
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
			} else {
				go func() {
					conn := connectionwrapper.New(tconn)
					ctx := context2.NewLayer4Context("tcp", conn, l.m)
					conn.StartRWLoop(ctx)
				}()
			}
		}
	}
}

func (l *listener) Stop() error {
	var err error
	switch l.protocol {
	case "udp":
		err = l.packetConn.SetDeadline(time.Now())
	case "tcp":
		err = l.limitListener.Listener.(*net.TCPListener).SetDeadline(time.Now())
	}
	return err
}

func (l *listener) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.limitListener != nil {
		return l.limitListener.Close()
	}
	if l.packetConn != nil {
		return l.packetConn.Close()
	}
	return nil
}
