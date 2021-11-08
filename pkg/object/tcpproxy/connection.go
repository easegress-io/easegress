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
	"io"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/limitlistener"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

const writeBufSize = 8

var tcpBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, iobufferpool.DefaultBufferReadCapacity)
		return buf
	},
}

// Connection wrap tcp connection
type Connection struct {
	closed     uint32
	rawConn    net.Conn
	localAddr  net.Addr
	remoteAddr net.Addr

	lastBytesSizeRead  int
	lastWriteSizeWrite int

	readBuffer      []byte
	writeBuffers    net.Buffers
	ioBuffers       []*iobufferpool.StreamBuffer
	writeBufferChan chan *iobufferpool.StreamBuffer

	mu               sync.Mutex
	connStopChan     chan struct{} // use for connection close
	listenerStopChan chan struct{} // use for listener close

	lastReadDeadlineTime  time.Time
	lastWriteDeadlineTime time.Time

	onRead  func(buffer *iobufferpool.StreamBuffer) // execute read filters
	onClose func(event ConnectionEvent)
}

// NewClientConn wrap connection create from client
func NewClientConn(conn net.Conn, listenerStopChan chan struct{}) *Connection {
	return &Connection{
		rawConn:          conn,
		localAddr:        conn.LocalAddr(),
		remoteAddr:       conn.RemoteAddr(),
		listenerStopChan: listenerStopChan,

		mu:              sync.Mutex{},
		connStopChan:    make(chan struct{}),
		writeBufferChan: make(chan *iobufferpool.StreamBuffer, writeBufSize),
	}
}

// SetOnRead set connection read handle
func (c *Connection) SetOnRead(onRead func(buffer *iobufferpool.StreamBuffer)) {
	c.onRead = onRead
}

// SetOnClose set close callback
func (c *Connection) SetOnClose(onclose func(event ConnectionEvent)) {
	c.onClose = onclose
}

// Start running connection read/write loop
func (c *Connection) Start() {
	fnRecover := func() {
		if r := recover(); r != nil {
			logger.Errorf("tcp connection goroutine panic: %v\n%s\n", r, string(debug.Stack()))
			c.Close(NoFlush, LocalClose)
		}
	}

	go func() {
		defer fnRecover()
		c.startReadLoop()
	}()

	go func() {
		defer fnRecover()
		c.startWriteLoop()
	}()
}

// Write receive other connection data
func (c *Connection) Write(buf *iobufferpool.StreamBuffer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("tcp connection has closed, local addr: %s, remote addr: %s, err: %+v",
				c.localAddr.String(), c.remoteAddr.String(), r)
			err = ErrConnectionHasClosed
		}
	}()

	select {
	case c.writeBufferChan <- buf:
		return
	default:
	}

	// try to send data again in 60 seconds
	t := timerpool.Get(60 * time.Second)
	select {
	case c.writeBufferChan <- buf:
	case <-t.C:
		buf.Release()
		err = ErrWriteBufferChanTimeout
	}
	timerpool.Put(t)
	return
}

func (c *Connection) startReadLoop() {
	defer func() {
		if c.readBuffer != nil {
			tcpBufferPool.Put(c.readBuffer[:iobufferpool.DefaultBufferReadCapacity])
		}
	}()

	for {
		select {
		case <-c.connStopChan:
			return
		case <-c.listenerStopChan:
			logger.Debugf("connection close due to listener stopped, local addr: %s, remote addr: %s",
				c.localAddr.String(), c.remoteAddr.String())
			c.Close(NoFlush, LocalClose)
			return
		default:
			n, err := c.doReadIO()
			if n > 0 {
				c.onRead(iobufferpool.NewStreamBuffer(c.readBuffer[:n]))
			}

			if err == nil {
				continue
			}

			if atomic.LoadUint32(&c.closed) == 1 {
				// connection has closed, so there is no need to record error log(error may create by CloseRead)
				logger.Debugf("connection has closed, exit read loop, local addr: %s, remote addr: %s",
					c.localAddr.String(), c.remoteAddr.String())
				return
			}

			if te, ok := err.(net.Error); ok && te.Timeout() {
				continue // ignore timeout error, read more stream data
			}

			if err == io.EOF {
				logger.Debugf("remote close connection, local addr: %s, remote addr: %s, err: %s",
					c.localAddr.String(), c.remoteAddr.String(), err.Error())
				c.Close(NoFlush, RemoteClose)
			} else {
				logger.Errorf("error on read, local addr: %s, remote addr: %s, err: %s",
					c.localAddr.String(), c.remoteAddr.String(), err.Error())
				c.Close(NoFlush, OnReadErrClose)
			}
			return
		}
	}
}

func (c *Connection) startWriteLoop() {
	var err error
	for {
		select {
		case <-c.connStopChan:
			logger.Debugf("connection exit write loop, local addr: %s, remote addr: %s",
				c.localAddr.String(), c.remoteAddr.String())
			return
		case buf, ok := <-c.writeBufferChan:
			if !ok {
				return
			}
			c.appendBuffer(buf)
		OUTER:
			// Keep reading until writeBufferChan is empty
			// writeBufferChan may be full when writeLoop call doWrite
			for i := 0; i < writeBufSize-1; i++ {
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
		}

		if _, err = c.doWrite(); err == nil {
			continue
		}

		if te, ok := err.(net.Error); ok && te.Timeout() {
			c.Close(NoFlush, OnWriteTimeout)
			return
		}

		if err == iobufferpool.ErrEOF {
			logger.Debugf("finish write with eof, local addr: %s, remote addr: %s",
				c.localAddr.String(), c.remoteAddr.String())
			c.Close(NoFlush, LocalClose)
		} else {
			// remote call CloseRead, so just exit write loop, wait read loop exit
			logger.Errorf("error on write, local addr: %s, remote addr: %s, err: %+v",
				c.localAddr.String(), c.remoteAddr.String(), err)
		}
		return
	}
}

func (c *Connection) appendBuffer(buf *iobufferpool.StreamBuffer) {
	if buf == nil {
		return
	}
	c.ioBuffers = append(c.ioBuffers, buf)
	c.writeBuffers = append(c.writeBuffers, buf.Bytes())
}

// Close connection close function
func (c *Connection) Close(ccType CloseType, event ConnectionEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("connection close panic, err: %+v\n%s", r, string(debug.Stack()))
		}
	}()

	if ccType == FlushWrite {
		_ = c.Write(iobufferpool.NewEOFStreamBuffer())
		return
	}

	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return
	}
	// close tcp rawConn read first
	logger.Debugf("enter connection close func(%s), local addr: %s, remote addr: %s",
		event, c.localAddr.String(), c.remoteAddr.String())

	// close tcp rawConn read first, make sure exit read loop
	if conn, ok := c.rawConn.(*limitlistener.Conn); ok {
		_ = conn.Conn.(*net.TCPConn).CloseRead() // client connection is wrapped by limitlistener.Conn
	} else {
		_ = c.rawConn.(*net.TCPConn).CloseRead()
	}

	close(c.connStopChan)
	_ = c.rawConn.Close()

	c.lastBytesSizeRead, c.lastWriteSizeWrite = 0, 0
	if c.onClose != nil {
		c.onClose(event)
	}
}

func (c *Connection) doReadIO() (bufLen int, err error) {
	if c.readBuffer == nil {
		c.readBuffer = tcpBufferPool.Get().([]byte)
	}

	// add read deadline setting optimization?
	// https://github.com/golang/go/issues/15133
	curr := fasttime.Now().Add(15 * time.Second)
	// there is no need to set readDeadline in too short time duration
	if diff := curr.Sub(c.lastReadDeadlineTime).Milliseconds(); diff > 0 {
		_ = c.rawConn.SetReadDeadline(curr)
		c.lastReadDeadlineTime = curr
	}
	return c.rawConn.(io.Reader).Read(c.readBuffer)
}

func (c *Connection) doWrite() (int64, error) {
	curr := fasttime.Now().Add(15 * time.Second)
	// there is no need to set writeDeadline in too short time duration
	if diff := curr.Sub(c.lastWriteDeadlineTime).Milliseconds(); diff > 0 {
		_ = c.rawConn.SetWriteDeadline(curr)
		c.lastWriteDeadlineTime = curr
	}
	bytesSent, err := c.doWriteIO()
	if err != nil {
		return 0, err
	}

	if bytesBufSize := c.writeBufLen(); bytesBufSize != c.lastWriteSizeWrite {
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
		buf.Release()
	}
	c.ioBuffers = c.ioBuffers[:0]
	c.writeBuffers = c.writeBuffers[:0]
	return
}

// ServerConnection wrap tcp connection to backend server
type ServerConnection struct {
	Connection
	connectTimeout time.Duration
}

// NewServerConn construct tcp server connection
func NewServerConn(connectTimeout uint32, serverAddr net.Addr, listenerStopChan chan struct{}) *ServerConnection {
	conn := &ServerConnection{
		Connection: Connection{
			remoteAddr: serverAddr,

			writeBufferChan: make(chan *iobufferpool.StreamBuffer, writeBufSize),

			mu:               sync.Mutex{},
			connStopChan:     make(chan struct{}),
			listenerStopChan: listenerStopChan,
		},
		connectTimeout: time.Duration(connectTimeout) * time.Millisecond,
	}
	return conn
}

// Connect create backend server tcp connection
func (u *ServerConnection) Connect() bool {
	addr := u.remoteAddr
	if addr == nil {
		logger.Errorf("cannot connect because the server has been closed, server addr: %s", addr.String())
		return false
	}

	timeout := u.connectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	var err error
	u.rawConn, err = net.DialTimeout("tcp", addr.String(), timeout)
	if err != nil {
		if err == io.EOF {
			logger.Errorf("cannot connect because the server has been closed, server addr: %s", addr.String())
		} else if te, ok := err.(net.Error); ok && te.Timeout() {
			logger.Errorf("connect to server timeout, server addr: %s", addr.String())
		} else {
			logger.Errorf("connect to server failed, server addr: %s, err: %s", addr.String(), err.Error())
		}
		return false
	}

	u.localAddr = u.rawConn.LocalAddr()
	_ = u.rawConn.(*net.TCPConn).SetNoDelay(true)
	_ = u.rawConn.(*net.TCPConn).SetKeepAlive(true)
	u.Start()
	return true
}
