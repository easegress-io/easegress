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

var tcpBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, iobufferpool.DefaultBufferReadCapacity)
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
	startOnce        sync.Once
	connStopChan     chan struct{} // use for connection close
	listenerStopChan chan struct{} // use for listener close

	onRead  func(buffer *iobufferpool.StreamBuffer) // execute read filters
	onClose func(event ConnectionEvent)
}

// NewClientConn wrap connection create from client
func NewClientConn(conn net.Conn, listenerStopChan chan struct{}) *Connection {
	return &Connection{
		rawConn:    conn,
		localAddr:  conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),

		writeBufferChan: make(chan *iobufferpool.StreamBuffer, 8),

		mu:               sync.Mutex{},
		connStopChan:     make(chan struct{}),
		listenerStopChan: listenerStopChan,
	}
}

// SetOnRead set connection read handle
func (c *Connection) SetOnRead(onRead func(buffer *iobufferpool.StreamBuffer)) {
	c.onRead = onRead
}

// OnRead set data read callback
func (c *Connection) OnRead(buffer *iobufferpool.StreamBuffer) {
	c.onRead(buffer)
}

// SetOnClose set close callback
func (c *Connection) SetOnClose(onclose func(event ConnectionEvent)) {
	c.onClose = onclose
}

// Start running connection read/write loop
func (c *Connection) Start() {
	c.startOnce.Do(func() {
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
	})
}

func (c *Connection) goWithRecover(handler func(), recoverHandler func(r interface{})) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("tcp connection goroutine panic: %v\n%s\n", r, string(debug.Stack()))
				if recoverHandler != nil {
					// it is not needed to wrap recoverHandler with go func in the current scenario
					recoverHandler(r)
				}
			}
		}()
		handler()
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
	for {
		select {
		case <-c.connStopChan:
			return
		case <-c.listenerStopChan:
			return
		default:
			n, err := c.doReadIO()
			if err != nil {
				if atomic.LoadUint32(&c.closed) == 1 {
					logger.Infof("tcp connection exit read loop for connection has closed, local addr: %s, "+
						"remote addr: %s, err: %s", c.localAddr.String(), c.remoteAddr.String(), err.Error())
					tcpBufferPool.Put(c.readBuffer)
					return
				}

				if te, ok := err.(net.Error); ok && te.Timeout() {
					if n == 0 {
						continue // continue read data, ignore timeout error
					}
				}
			}

			if n != 0 && (err == nil || err == io.EOF) {
				c.onRead(iobufferpool.NewStreamBuffer(c.readBuffer[:n]))
			}

			if err != nil {
				if err == io.EOF {
					logger.Infof("tcp connection read error, local addr: %s, remote addr: %s, err: %s",
						c.localAddr.String(), c.remoteAddr.String(), err.Error())
					_ = c.Close(NoFlush, RemoteClose)
				} else {
					logger.Errorf("tcp connection read error, local addr: %s, remote addr: %s, err: %s",
						c.localAddr.String(), c.remoteAddr.String(), err.Error())
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

func (c *Connection) appendBuffer(buf *iobufferpool.StreamBuffer) {
	if buf == nil {
		return
	}
	c.ioBuffers = append(c.ioBuffers, buf)
	c.writeBuffers = append(c.writeBuffers, buf.Bytes())
}

// Close connection close function
func (c *Connection) Close(ccType CloseType, event ConnectionEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("tcp connection close panic, err: %+v\n%s", r, string(debug.Stack()))
		}
	}()

	if ccType == FlushWrite {
		_ = c.Write(iobufferpool.NewEOFStreamBuffer())
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

func (c *Connection) doReadIO() (bufLen int, err error) {
	if c.readBuffer == nil {
		c.readBuffer = tcpBufferPool.Get().([]byte)[:iobufferpool.DefaultBufferReadCapacity]
	}

	// add read deadline setting optimization?
	// https://github.com/golang/go/issues/15133
	_ = c.rawConn.SetReadDeadline(time.Now().Add(15 * time.Second))
	return c.rawConn.(io.Reader).Read(c.readBuffer)
}

func (c *Connection) doWrite() (int64, error) {
	bytesSent, err := c.doWriteIO()
	if err != nil && atomic.LoadUint32(&c.closed) == 1 {
		return 0, nil
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
	connectOnce    sync.Once
}

// NewServerConn construct tcp server connection
func NewServerConn(connectTimeout uint32, serverAddr net.Addr, listenerStopChan chan struct{}) *ServerConnection {
	conn := &ServerConnection{
		Connection: Connection{
			remoteAddr: serverAddr,

			writeBufferChan: make(chan *iobufferpool.StreamBuffer, 8),

			mu:               sync.Mutex{},
			connStopChan:     make(chan struct{}),
			listenerStopChan: listenerStopChan,
		},
		connectTimeout: time.Duration(connectTimeout) * time.Millisecond,
	}
	return conn
}

// Connect create backend server tcp connection
func (u *ServerConnection) Connect() (err error) {
	u.connectOnce.Do(func() {
		addr := u.remoteAddr
		if addr == nil {
			err = errors.New("server addr is nil")
			return
		}

		timeout := u.connectTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}

		u.rawConn, err = net.DialTimeout("tcp", addr.String(), timeout)
		if err != nil {
			if err == io.EOF {
				err = errors.New("server has been closed")
			} else if te, ok := err.(net.Error); ok && te.Timeout() {
				err = errors.New("connect to server timeout")
			} else {
				err = errors.New("connect to server failed")
			}
			return
		}

		u.localAddr = u.rawConn.LocalAddr()
		_ = u.rawConn.(*net.TCPConn).SetNoDelay(true)
		_ = u.rawConn.(*net.TCPConn).SetKeepAlive(true)
		u.Start()
	})
	return
}
