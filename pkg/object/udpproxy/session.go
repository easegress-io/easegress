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
)

type session struct {
	upstreamAddr          string
	downstreamAddr        *net.UDPAddr
	downstreamIdleTimeout time.Duration
	upstreamIdleTimeout   time.Duration

	upstreamConn net.Conn
	writeBuf     chan *iobufferpool.IoBuffer
	stopChan     chan struct{}
	stopped      uint32
}

func newSession(downstreamAddr *net.UDPAddr, upstreamAddr string, upstreamConn net.Conn,
	downstreamIdleTimeout, upstreamIdleTimeout time.Duration) *session {
	s := session{
		upstreamAddr:          upstreamAddr,
		downstreamAddr:        downstreamAddr,
		upstreamConn:          upstreamConn,
		upstreamIdleTimeout:   upstreamIdleTimeout,
		downstreamIdleTimeout: downstreamIdleTimeout,

		writeBuf: make(chan *iobufferpool.IoBuffer, 512),
		stopChan: make(chan struct{}, 1),
	}

	go func() {
		var t *time.Timer
		var idleCheck <-chan time.Time

		if downstreamIdleTimeout > 0 {
			t = time.NewTimer(downstreamIdleTimeout)
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
					t.Reset(downstreamIdleTimeout)
				}

				bufLen := (*buf).Len()
				n, err := s.upstreamConn.Write((*buf).Bytes())
				_ = iobufferpool.PutIoBuffer(*buf)

				if err != nil {
					logger.Errorf("udp connection flush data to upstream(%s) failed, err: %+v", upstreamAddr, err)
					s.Close()
					continue
				}

				if bufLen != n {
					logger.Errorf("udp connection flush data to upstream(%s) failed, should write %d but written %d",
						upstreamAddr, bufLen, n)
					s.Close()
				}
			case <-s.stopChan:
				if !atomic.CompareAndSwapUint32(&s.stopped, 0, 1) {
					break
				}
				if t != nil {
					t.Stop()
				}
				_ = s.upstreamConn.Close()
				s.cleanWriteBuf()
			}
		}
	}()

	return &s
}

// Write send data to buffer channel, wait flush to upstream
func (s *session) Write(buf *iobufferpool.IoBuffer) error {
	if atomic.LoadUint32(&s.stopped) == 1 {
		return fmt.Errorf("udp connection from %s to %s has closed", s.downstreamAddr.String(), s.upstreamAddr)
	}

	select {
	case s.writeBuf <- buf:
	default:
		_ = iobufferpool.PutIoBuffer(*buf) // if failed, may be try again?
	}
	return nil
}

// ListenResponse session listen upstream connection response and send to downstream
func (s *session) ListenResponse(sendTo *net.UDPConn) {
	go func() {
		buf := iobufferpool.GetIoBuffer(iobufferpool.UDPPacketMaxSize)
		defer s.Close()

		for {
			buf.Reset()
			if s.upstreamIdleTimeout > 0 {
				_ = s.upstreamConn.SetReadDeadline(time.Now().Add(s.upstreamIdleTimeout))
			}

			nRead, err := buf.ReadOnce(s.upstreamConn)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return
				}

				if atomic.LoadUint32(&s.stopped) == 0 {
					logger.Errorf("udp connection read data from upstream(%s) failed, err: %+v", s.upstreamAddr, err)
				}
				return
			}

			nWrite, err := sendTo.WriteToUDP(buf.Bytes(), s.downstreamAddr)
			if err != nil {
				logger.Errorf("udp connection send data to downstream(%s) failed, err: %+v", s.downstreamAddr.String(), err)
				return
			}

			if nRead != int64(nWrite) {
				logger.Errorf("udp connection send data to downstream(%s) failed, should write %d but written %d",
					s.downstreamAddr.String(), nRead, nWrite)
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
				_ = iobufferpool.PutIoBuffer(*buf)
			}
		default:
			return
		}
	}
}

// IsClosed determine session if it is closed
func (s *session) IsClosed() bool {
	return atomic.LoadUint32(&s.stopped) == 1
}

// Close send session close signal
func (s *session) Close() {
	select {
	case s.stopChan <- struct{}{}:
	default:
	}
}
