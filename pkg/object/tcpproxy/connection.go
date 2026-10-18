/*
 * Copyright (c) 2017, The Easegress Authors
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
	"crypto/tls"
	"errors"
	"io"
	stdnet "net"
	"sync"
	"sync/atomic"
	"time"
)

// stdTLSConn wraps a connection to perform TLS handshake.
type stdTLSConn struct {
	stdnet.Conn
	config  *tls.Config
	tlsConn *tls.Conn
	once    sync.Once
}

func (c *stdTLSConn) handshake() error {
	var err error
	c.once.Do(func() {
		c.tlsConn = tls.Server(c.Conn, c.config)
		err = c.tlsConn.Handshake()
	})
	return err
}

func (c *stdTLSConn) Read(b []byte) (int, error) {
	if c.tlsConn == nil {
		return 0, io.EOF
	}
	return c.tlsConn.Read(b)
}

func (c *stdTLSConn) Write(b []byte) (int, error) {
	if c.tlsConn == nil {
		return 0, io.EOF
	}
	return c.tlsConn.Write(b)
}

func (c *stdTLSConn) Close() error {
	if c.tlsConn != nil {
		return c.tlsConn.Close()
	}
	return c.Conn.Close()
}

// proxy copies data bidirectionally between two connections with optional
// timeouts.
//
// idleTimeout is connection-wide: activity in either direction resets the
// idle timer, so a single-direction stream (e.g. server-sent events) is not
// falsely terminated while the reverse direction is quiet.
//
// sendTimeout and receiveTimeout are applied as per-I/O write and read
// deadlines respectively; both are enforced independently of the idle policy.
// receiveTimeout fires regardless of whether idleTimeout is also configured.
//
// When one copy direction reaches a clean EOF, a half-close is propagated to
// the peer so the reverse direction can still deliver its response before the
// proxy tears down. Terminal I/O errors (including true idle timeouts) close
// both sockets so the other goroutine unblocks promptly.
func proxy(src, dst stdnet.Conn, idleTimeout, sendTimeout, receiveTimeout time.Duration) {
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())

	// closeBothOnce closes both connections exactly once. Used only for
	// terminal errors or true connection-wide idle timeouts.
	closeBothOnce := sync.Once{}
	closeBoth := func() {
		closeBothOnce.Do(func() {
			_ = src.Close()
			_ = dst.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// halfClose attempts a graceful half-close on the write side of conn,
	// which is correct for propagating clean EOF (the peer closed its write
	// side but may still be sending in the reverse direction). Falls back to
	// a full close if the concrete type does not support CloseWrite.
	halfClose := func(c stdnet.Conn) {
		if cw, ok := c.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
			return
		}
		_ = c.Close()
	}

	// nextReadDeadline returns the next absolute time.Time deadline for a
	// copyOne goroutine, or time.Time{} when no deadline should be set
	// (blocking read). It combines three policies:
	//   - receiveTimeout: hard per-direction receive deadline, always honored.
	//   - idleTimeout: connection-wide idle interval — poll at
	//     min(receiveTimeout, remaining_idle_interval) so we never sleep past
	//     either deadline. Cap poll at 1s when both are generous to avoid
	//     starving other goroutines.
	nextReadDeadline := func() time.Time {
		now := time.Now()
		var d time.Duration

		if receiveTimeout > 0 {
			d = receiveTimeout
		}

		if idleTimeout > 0 {
			idleLeft := idleTimeout - now.Sub(time.Unix(0, lastActivity.Load()))
			if idleLeft <= 0 {
				// Already past idle — return a past time so the timeout
				// handler closes the connection unconditionally.
				return now.Add(-1 * time.Nanosecond)
			}
			if d == 0 || idleLeft < d {
				d = idleLeft
			}
			// Cap at 1s so we do not starve other goroutines when both
			// timeouts are generous.
			if d > 1*time.Second {
				d = 1 * time.Second
			}
		}

		if d == 0 {
			return time.Time{} // no deadline — blocking read
		}
		return now.Add(d)
	}

	copyOne := func(from, to stdnet.Conn) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			dl := nextReadDeadline()
			_ = from.SetReadDeadline(dl)
			n, err := from.Read(buf)
			if n > 0 {
				lastActivity.Store(time.Now().UnixNano())
				if sendTimeout > 0 {
					_ = to.SetWriteDeadline(time.Now().Add(sendTimeout))
				} else {
					_ = to.SetWriteDeadline(time.Time{})
				}
				if _, werr := to.Write(buf[:n]); werr != nil {
					closeBoth() // write error is terminal — unblock the other goroutine.
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					// Peer closed its write side (clean EOF). Propagate a
					// half-close on our write side so the peer knows to stop
					// sending, but keep the reverse direction open to let
					// any remaining response traffic through.
					halfClose(to)
					return
				}
				if netErr, ok := err.(stdnet.Error); ok && netErr.Timeout() {
					// Read deadline exceeded. Check three conditions in order:
					// (1) receiveTimeout was configured and has fired — terminate
					//     regardless of idle policy (receiveTimeout is independent).
					// (2) idleTimeout is configured and the connection-wide idle
					//     interval has elapsed — terminate.
					// (3) otherwise this was a poll wakeup for idle checking;
					//     fall through and reset the deadline.
					if receiveTimeout > 0 && n == 0 {
						closeBoth()
						return
					}
					if idleTimeout > 0 && time.Since(time.Unix(0, lastActivity.Load())) > idleTimeout {
						closeBoth()
						return
					}
					// Poll wakeup for idle check — reset and retry.
					continue
				}
				// Terminal I/O error — tear both sockets down to unblock the
				// other goroutine promptly.
				closeBoth()
				return
			}
		}
	}

	go copyOne(src, dst)
	go copyOne(dst, src)

	wg.Wait()
}
