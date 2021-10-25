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
	"errors"
	"io"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

// Connection wrap tcp connection
type Connection struct {
	rawConn   net.Conn
	connected uint32
	closed    uint32

	localAddr  net.Addr
	remoteAddr net.Addr

	lastBytesSizeRead  int64
	lastWriteSizeWrite int64

	readBuffer      iobufferpool.IoBuffer
	writeBuffers    net.Buffers
	ioBuffers       []iobufferpool.IoBuffer
	writeBufferChan chan *[]iobufferpool.IoBuffer

	mu               sync.Mutex
	startOnce        sync.Once
	connStopChan     chan struct{} // use for connection close
	listenerStopChan chan struct{} // use for listener close

	onRead  func(buffer iobufferpool.IoBuffer) // execute read filters
	onClose func(event ConnectionEvent)
}

// NewDownstreamConn wrap connection create from client
// @param remoteAddr client addr for udp proxy use
func NewDownstreamConn(conn net.Conn, remoteAddr net.Addr, listenerStopChan chan struct{}) *Connection {
	clientConn := &Connection{
		connected:  1,
		rawConn:    conn,
		localAddr:  conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),

		writeBufferChan: make(chan *[]iobufferpool.IoBuffer, 8),

		mu:               sync.Mutex{},
		connStopChan:     make(chan struct{}),
		listenerStopChan: listenerStopChan,
	}

	if remoteAddr != nil {
		clientConn.remoteAddr = remoteAddr
	} else {
		clientConn.remoteAddr = conn.RemoteAddr() // udp server rawConn can not get remote address
	}
	return clientConn
}

// LocalAddr get connection local addr
func (c *Connection) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr get connection remote addr(it's nil for udp server rawConn)
func (c *Connection) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

// SetOnRead set connection read handle
func (c *Connection) SetOnRead(onRead func(buffer iobufferpool.IoBuffer)) {
	c.onRead = onRead
}

// OnRead set data read callback
func (c *Connection) OnRead(buffer iobufferpool.IoBuffer) {
	c.onRead(buffer)
}

// SetOnClose set close callback
func (c *Connection) SetOnClose(onclose func(event ConnectionEvent)) {
	c.onClose = onclose
}

// GetReadBuffer get connection read buffer
func (c *Connection) GetReadBuffer() iobufferpool.IoBuffer {
	return c.readBuffer
}

// Start running connection read/write loop
func (c *Connection) Start() {
	c.startOnce.Do(func() {
		c.startRWLoop()
	})
}

// State get connection running state
func (c *Connection) State() ConnState {
	if atomic.LoadUint32(&c.closed) == 1 {
		return ConnClosed
	}
	if atomic.LoadUint32(&c.connected) == 1 {
		return ConnActive
	}
	return ConnInit
}

// GoWithRecover wraps a `go func()` with recover()
func (c *Connection) goWithRecover(handler func(), recoverHandler func(r interface{})) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("tcp connection goroutine panic: %v\n%s\n", r, string(debug.Stack()))
				if recoverHandler != nil {
					go func() {
						defer func() {
							if p := recover(); p != nil {
								logger.Errorf("tcp connection goroutine panic: %v\n%s\n", p, string(debug.Stack()))
							}
						}()
						recoverHandler(r)
					}()
				}
			}
		}()
		handler()
	}()
}

func (c *Connection) startRWLoop() {
	c.goWithRecover(func() {
		c.startReadLoop()
	}, func(r interface{}) {
		_ = c.Close(NoFlush, LocalClose)
	})

	c.goWithRecover(func() {
		c.startWriteLoop()
	}, func(r interface{}) {
		_ = c.Close(NoFlush, LocalClose)
	})
}

// Write receive other connection data
func (c *Connection) Write(buffers ...iobufferpool.IoBuffer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("tcp connection has closed, local addr: %s, remote addr: %s, err: %+v",
				c.localAddr.String(), c.remoteAddr.String(), r)
			err = ErrConnectionHasClosed
		}
	}()

	select {
	case c.writeBufferChan <- &buffers:
		return
	default:
	}

	// try to send data again in 60 seconds
	t := timerpool.Get(60 * time.Second)
	select {
	case c.writeBufferChan <- &buffers:
	case <-t.C:
		err = ErrWriteBufferChanTimeout
	}
	timerpool.Put(t)
	return
}

func (c *Connection) startReadLoop() {
	for {
		select {
		case <-c.connStopChan:
			return
		case <-c.listenerStopChan:
			return
		default:
			err := c.doReadIO()
			if err != nil {
				if te, ok := err.(net.Error); ok && te.Timeout() {
					if c.readBuffer != nil && c.readBuffer.Len() == 0 && c.readBuffer.Cap() > iobufferpool.DefaultBufferReadCapacity {
						c.readBuffer.Free()
						c.readBuffer.Alloc(iobufferpool.DefaultBufferReadCapacity)
					}
					continue
				}

				// normal close or health check
				if c.lastBytesSizeRead == 0 || err == io.EOF {
					logger.Infof("tcp connection error on read, local addr: %s, remote addr: %s, err: %s",
						c.localAddr.String(), c.remoteAddr.String(), err.Error())
				} else {
					logger.Errorf("tcp connection error on read, local addr: %s, remote addr: %s, err: %s",
						c.localAddr.String(), c.remoteAddr.String(), err.Error())
				}

				if err == io.EOF {
					_ = c.Close(NoFlush, RemoteClose)
				} else {
					_ = c.Close(NoFlush, OnReadErrClose)
				}
				return
			}
		}
	}
}

func (c *Connection) startWriteLoop() {
	var err error
	for {
		select {
		case <-c.listenerStopChan:
			return
		case buf, ok := <-c.writeBufferChan:
			if !ok {
				return
			}
			c.appendBuffer(buf)
		OUTER:
			for i := 0; i < 8; i++ {
				select {
				case buf, ok := <-c.writeBufferChan:
					if !ok {
						return
					}
					c.appendBuffer(buf)
				default:
					break OUTER
				}
			}

			_ = c.rawConn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			_, err = c.doWrite()
		}

		if err != nil {
			if err == iobufferpool.ErrEOF {
				logger.Debugf("tcp connection local close with eof, local addr: %s, remote addr: %s",
					c.localAddr.String(), c.remoteAddr.String())
				_ = c.Close(NoFlush, LocalClose)
			} else {
				logger.Errorf("tcp connection error on write, local addr: %s, remote addr: %s, err: %+v",
					c.localAddr.String(), c.remoteAddr.String(), err)
			}

			if te, ok := err.(net.Error); ok && te.Timeout() {
				_ = c.Close(NoFlush, OnWriteTimeout)
			}
			//other write errs not close connection, because readbuffer may have unread data, wait for readloop close connection,
			return
		}
	}
}

func (c *Connection) appendBuffer(ioBuffers *[]iobufferpool.IoBuffer) {
	if ioBuffers == nil {
		return
	}
	for _, buf := range *ioBuffers {
		if buf == nil {
			continue
		}
		c.ioBuffers = append(c.ioBuffers, buf)
		c.writeBuffers = append(c.writeBuffers, buf.Bytes())
	}
}

// Close connection close function
func (c *Connection) Close(ccType CloseType, event ConnectionEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("tcp connection close panic, err: %+v\n%s", r, string(debug.Stack()))
		}
	}()

	if ccType == FlushWrite {
		_ = c.Write(iobufferpool.NewIoBufferEOF())
		return nil
	}

	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return nil
	}

	// connection failed in client mode
	if c.rawConn == nil || reflect.ValueOf(c.rawConn).IsNil() {
		return nil
	}

	// close tcp rawConn read first
	if tconn, ok := c.rawConn.(*net.TCPConn); ok {
		logger.Debugf("tcp connection closed, local addr: %s, remote addr: %s, event: %s",
			c.localAddr.String(), c.remoteAddr.String(), event)
		_ = tconn.CloseRead()
	}

	// close rawConn recv, then notify read/write loop to exit
	close(c.connStopChan)
	_ = c.rawConn.Close()
	c.lastBytesSizeRead, c.lastWriteSizeWrite = 0, 0

	logger.Debugf("tcp connection closed, local addr: %s, remote addr: %s, event: %s", c.localAddr.String(), c.remoteAddr.String(), event)

	if c.onClose != nil {
		c.onClose(event)
	}
	return nil
}

func (c *Connection) doReadIO() (err error) {
	if c.readBuffer == nil {
		c.readBuffer = iobufferpool.GetIoBuffer(iobufferpool.DefaultBufferReadCapacity)
	}

	var bytesRead int64
	_ = c.rawConn.SetReadDeadline(time.Now().Add(15 * time.Second))
	bytesRead, err = c.readBuffer.ReadOnce(c.rawConn)

	if err != nil {
		if atomic.LoadUint32(&c.closed) == 1 {
			return err
		}
		if te, ok := err.(net.Error); ok && te.Timeout() {
			if bytesRead == 0 {
				return err
			}
		} else if err != io.EOF {
			return err
		}
	}

	if bytesRead == 0 && err == nil {
		err = io.EOF
		logger.Errorf("tcp connection ReadOnce maybe always return (0, nil) and causes dead loop, local addr: %s, remote addr: %s",
			c.localAddr.String(), c.remoteAddr.String())
	}

	if c.readBuffer.Len() == 0 {
		return
	}

	c.onRead(c.readBuffer)
	if currLen := int64(c.readBuffer.Len()); c.lastBytesSizeRead != currLen {
		c.lastBytesSizeRead = currLen
	}
	return
}

func (c *Connection) doWrite() (int64, error) {
	bytesSent, err := c.doWriteIO()
	if err != nil && atomic.LoadUint32(&c.closed) == 1 {
		return 0, nil
	}

	if bytesBufSize := int64(c.writeBufLen()); bytesBufSize != c.lastWriteSizeWrite {
		c.lastWriteSizeWrite = bytesBufSize
	}
	return bytesSent, err
}

func (c *Connection) writeBufLen() (bufLen int) {
	for _, buf := range c.writeBuffers {
		bufLen += len(buf)
	}
	return
}

func (c *Connection) doWriteIO() (bytesSent int64, err error) {
	buffers := c.writeBuffers
	bytesSent, err = buffers.WriteTo(c.rawConn)

	if err != nil {
		return bytesSent, err
	}
	for i, buf := range c.ioBuffers {
		c.ioBuffers[i] = nil
		c.writeBuffers[i] = nil
		if buf.EOF() {
			err = iobufferpool.ErrEOF
		}
		if e := iobufferpool.PutIoBuffer(buf); e != nil {
			logger.Errorf("tcp connection give buffer error, local addr: %s, remote addr: %s, err: %+v",
				c.localAddr.String(), c.remoteAddr.String(), err)
		}
	}
	c.ioBuffers = c.ioBuffers[:0]
	c.writeBuffers = c.writeBuffers[:0]
	return
}

// UpstreamConnection wrap connection to upstream
type UpstreamConnection struct {
	Connection
	connectTimeout time.Duration
	connectOnce    sync.Once
}

// NewUpstreamConn construct tcp upstream connection
func NewUpstreamConn(connectTimeout uint32, upstreamAddr net.Addr, listenerStopChan chan struct{}) *UpstreamConnection {
	conn := &UpstreamConnection{
		Connection: Connection{
			connected:  1,
			remoteAddr: upstreamAddr,

			writeBufferChan: make(chan *[]iobufferpool.IoBuffer, 8),

			mu:               sync.Mutex{},
			connStopChan:     make(chan struct{}),
			listenerStopChan: listenerStopChan,
		},
		connectTimeout: time.Duration(connectTimeout) * time.Millisecond,
	}
	return conn
}

func (u *UpstreamConnection) connect() (event ConnectionEvent, err error) {
	timeout := u.connectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	addr := u.remoteAddr
	if addr == nil {
		return ConnectFailed, errors.New("upstream addr is nil")
	}
	u.rawConn, err = net.DialTimeout("tcp", addr.String(), timeout)
	if err != nil {
		if err == io.EOF {
			event = RemoteClose
		} else if err, ok := err.(net.Error); ok && err.Timeout() {
			event = ConnectTimeout
		} else {
			event = ConnectFailed
		}
		return
	}
	atomic.StoreUint32(&u.connected, 1)
	u.localAddr = u.rawConn.LocalAddr()

	_ = u.rawConn.(*net.TCPConn).SetNoDelay(true)
	_ = u.rawConn.(*net.TCPConn).SetKeepAlive(true)
	event = Connected
	return
}

// Connect tcp upstream connect to backend server
func (u *UpstreamConnection) Connect() (err error) {
	u.connectOnce.Do(func() {
		var event ConnectionEvent
		event, err = u.connect()
		if err == nil {
			u.Start()
		}
		logger.Debugf("tcp connect upstream(%s), event: %s, err: %+v", u.remoteAddr, event, err)
	})
	return
}
