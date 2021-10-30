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
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

type (
	session struct {
		clientAddr        *net.UDPAddr
		serverAddr        string
		clientIdleTimeout time.Duration
		serverIdleTimeout time.Duration

		serverConn net.Conn
		writeBuf   chan *iobufferpool.Packet
		stopChan   chan struct{}
		stopped    uint32
	}
)

func newSession(clientAddr *net.UDPAddr, serverAddr string, serverConn net.Conn,
	clientIdleTimeout, serverIdleTimeout time.Duration) *session {
	s := session{
		serverAddr:        serverAddr,
		clientAddr:        clientAddr,
		serverConn:        serverConn,
		serverIdleTimeout: serverIdleTimeout,
		clientIdleTimeout: clientIdleTimeout,

		writeBuf: make(chan *iobufferpool.Packet, 512),
		stopChan: make(chan struct{}),
	}

	go func() {
		var t *time.Timer
		var idleCheck <-chan time.Time

		if clientIdleTimeout > 0 {
			t = time.NewTimer(clientIdleTimeout)
			idleCheck = t.C
		}

		for {
			select {
			case <-idleCheck:
				s.Close()
			case buf, ok := <-s.writeBuf:
				if !ok {
					s.Close()
					continue
				}

				if t != nil {
					if !t.Stop() {
						<-t.C
					}
					t.Reset(clientIdleTimeout)
				}

				bufLen := len(buf.Payload)
				n, err := s.serverConn.Write(buf.Bytes())
				buf.Release()

				if err != nil {
					logger.Errorf("udp connection flush data to server(%s) failed, err: %+v", serverAddr, err)
					s.Close()
					continue
				}

				if bufLen != n {
					logger.Errorf("udp connection flush data to server(%s) failed, should write %d but written %d",
						serverAddr, bufLen, n)
					s.Close()
				}
			case <-s.stopChan:
				if t != nil {
					t.Stop()
				}
				_ = s.serverConn.Close()
				s.cleanWriteBuf()
				return
			}
		}
	}()

	return &s
}

// Write send data to buffer channel, wait flush to server
func (s *session) Write(buf *iobufferpool.Packet) error {
	select {
	case s.writeBuf <- buf:
		return nil // try to send data with no check
	default:
	}

	var t *time.Timer
	if s.serverIdleTimeout != 0 {
		t = timerpool.Get(s.serverIdleTimeout * time.Millisecond)
	} else {
		t = timerpool.Get(60 * time.Second)
	}
	defer timerpool.Put(t)

	select {
	case s.writeBuf <- buf:
		return nil
	case <-s.stopChan:
		buf.Release()
		return nil
	case <-t.C:
		buf.Release()
		return fmt.Errorf("write data to channel timeout")
	}
}

// ListenResponse session listen server connection response and send to client
func (s *session) ListenResponse(sendTo *net.UDPConn) {
	go func() {
		buf := iobufferpool.UDPBufferPool.Get().([]byte)
		defer s.Close()

		for {
			if s.serverIdleTimeout > 0 {
				_ = s.serverConn.SetReadDeadline(time.Now().Add(s.serverIdleTimeout))
			}

			n, err := s.serverConn.Read(buf)
			if err != nil {
				select {
				case <-s.stopChan:
					return // if session has closed, exit
				default:
				}

				if err, ok := err.(net.Error); ok && err.Timeout() {
					continue
				}
				return
			}

			nWrite, err := sendTo.WriteToUDP(buf[0:n], s.clientAddr)
			if err != nil {
				logger.Errorf("udp connection send data to client(%s) failed, err: %+v", s.clientAddr.String(), err)
				return
			}

			if n != nWrite {
				logger.Errorf("udp connection send data to client(%s) failed, should write %d but written %d",
					s.clientAddr.String(), n, nWrite)
				return
			}
		}
	}()
}

func (s *session) cleanWriteBuf() {
	for {
		select {
		case buf := <-s.writeBuf:
			if buf != nil {
				buf.Release()
			}
		default:
			return
		}
	}
}

// isClosed determine session if it is closed, used only for clean sessionMap
func (s *session) isClosed() bool {
	return atomic.LoadUint32(&s.stopped) == 1
}

// Close send session close signal
func (s *session) Close() {
	if atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
		close(s.stopChan)
	}
}
