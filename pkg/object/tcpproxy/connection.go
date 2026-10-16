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

	copyOne := func(from, to stdnet.Conn) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			// Set a read deadline so we can detect true idle timeouts (when
			// traffic has stopped in *both* directions) and per-direction
			// receive deadlines independently.
			if receiveTimeout > 0 {
				_ = from.SetReadDeadline(time.Now().Add(receiveTimeout))
			} else if idleTimeout > 0 {
				// Poll frequently enough to check the shared activity stamp
				// so we do not miss connection-wide idleness when a single
				// direction goes quiet.
				_ = from.SetReadDeadline(time.Now().Add(1 * time.Second))
			}
			n, err := from.Read(buf)
			if n > 0 {
				lastActivity.Store(time.Now().UnixNano())
				if sendTimeout > 0 {
					_ = to.SetWriteDeadline(time.Now().Add(sendTimeout))
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
					// Read deadline exceeded — determine whether the *entire*
					// connection has been idle or only this direction has no
					// traffic while the other is active.
					if idleTimeout > 0 && time.Since(time.Unix(0, lastActivity.Load())) > idleTimeout {
						closeBoth()
						return
					}
					// Stale per-direction deadline; reset and try again.
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
