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
	"io"
	stdnet "net"
	"sync"
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

// proxyBidirectional copies data bidirectionally between two connections.
// It stops when either connection closes or encounters an error.
func proxyBidirectional(src, dst stdnet.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyFn := func(from, to stdnet.Conn) {
		defer wg.Done()
		defer to.Close()
		defer from.Close()
		_, _ = io.Copy(to, from)
	}

	go copyFn(src, dst)
	go copyFn(dst, src)

	wg.Wait()
}

// proxy copies data bidirectionally between two connections with optional idle timeout.
func proxy(src, dst stdnet.Conn, idleTimeout time.Duration) {
	if idleTimeout > 0 {
		// Set read deadlines to detect idle connections.
		src.SetReadDeadline(time.Now().Add(idleTimeout))
		dst.SetReadDeadline(time.Now().Add(idleTimeout))
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// src -> dst
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			if idleTimeout > 0 {
				src.SetReadDeadline(time.Now().Add(idleTimeout))
			}
			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					return
				}
				if idleTimeout > 0 {
					dst.SetReadDeadline(time.Now().Add(idleTimeout))
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// dst -> src
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			if idleTimeout > 0 {
				dst.SetReadDeadline(time.Now().Add(idleTimeout))
			}
			n, err := dst.Read(buf)
			if n > 0 {
				if _, werr := src.Write(buf[:n]); werr != nil {
					return
				}
				if idleTimeout > 0 {
					src.SetReadDeadline(time.Now().Add(idleTimeout))
				}
			}
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
}
