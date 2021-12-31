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
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

type (
	session struct {
		clientAddr        *net.UDPAddr
		serverAddr        string
		serverConn        net.Conn
		clientIdleTimeout time.Duration
		serverIdleTimeout time.Duration
		writeBuf          chan *iobufferpool.Packet

		stopped      bool
		stopChan     chan struct{}
		listenerStop chan struct{}
		onClose      func()

		mu sync.Mutex
	}
)

func newSession(clientAddr *net.UDPAddr, serverAddr string, serverConn net.Conn,
	listenerStop chan struct{}, onClose func(),
	clientIdleTimeout, serverIdleTimeout time.Duration) *session {
	s := session{
		serverAddr:        serverAddr,
		clientAddr:        clientAddr,
		serverConn:        serverConn,
		serverIdleTimeout: serverIdleTimeout,
		clientIdleTimeout: clientIdleTimeout,
		writeBuf:          make(chan *iobufferpool.Packet, 256),

		stopped:      false,
		stopChan:     make(chan struct{}),
		listenerStop: listenerStop,
		onClose:      onClose,
	}

	go s.startSession(serverAddr, clientIdleTimeout)
	return &s
}

func (s *session) startSession(serverAddr string, clientIdleTimeout time.Duration) {
	var t *time.Timer
	var idleCheck <-chan time.Time

	if clientIdleTimeout > 0 {
		t = time.NewTimer(clientIdleTimeout)
		idleCheck = t.C
	}

	for {
		select {
		case <-s.listenerStop:
			s.close()
		case <-idleCheck:
			s.close()
		case buf, ok := <-s.writeBuf:
			if !ok {
				s.close()
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
				s.close()
				continue
			}

			if bufLen != n {
				logger.Errorf("udp connection flush data to server(%s) failed, should write %d but written %d",
					serverAddr, bufLen, n)
				s.close()
			}
		case <-s.stopChan:
			if t != nil {
				t.Stop()
			}
			_ = s.serverConn.Close()
			s.cleanWriteBuf()
			s.onClose()
			return
		}
	}
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
		defer s.close()

		for {
			if s.serverIdleTimeout > 0 {
				_ = s.serverConn.SetReadDeadline(fasttime.Now().Add(s.serverIdleTimeout))
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

// close send session close signal
func (s *session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped == true {
		return
	}

	s.stopped = true
	s.onClose()
	close(s.stopChan)
}
