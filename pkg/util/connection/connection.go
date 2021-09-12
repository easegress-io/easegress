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

package connection

import (
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

type Connection struct {
	conn       net.Conn
	connected  uint32
	closed     uint32
	protocol   string
	localAddr  net.Addr
	remoteAddr net.Addr

	readEnabled     bool
	readEnabledChan chan bool // if we need to reload read filters, it's better to stop read data before reload filters

	lastBytesSizeRead  int64
	lastWriteSizeWrite int64
	readBuffer         iobufferpool.IoBuffer
	writeBuffers       net.Buffers
	ioBuffers          []iobufferpool.IoBuffer
	writeBufferChan    chan *[]iobufferpool.IoBuffer

	mu        sync.Mutex
	startOnce sync.Once
	stopChan  chan struct{}

	onRead  func(buffer iobufferpool.IoBuffer)                        // execute read filters
	onWrite func(src []iobufferpool.IoBuffer) []iobufferpool.IoBuffer // execute write filters
}

// NewClientConnection wrap connection create from client
func NewClientConnection(conn net.Conn, remoteAddr net.Addr, stopChan chan struct{}) *Connection {
	res := &Connection{
		conn:      conn,
		connected: 1,
		protocol:  conn.LocalAddr().Network(),
		localAddr: conn.LocalAddr(),

		readEnabled:     true,
		readEnabledChan: make(chan bool, 1),
		writeBufferChan: make(chan *[]iobufferpool.IoBuffer, 8),

		mu:       sync.Mutex{},
		stopChan: stopChan,
	}

	if remoteAddr != nil {
		res.remoteAddr = remoteAddr
	} else {
		res.remoteAddr = conn.RemoteAddr()
	}
	return res
}

func (c *Connection) Start() {
	if c.protocol == "udp" && c.conn.RemoteAddr() == nil {
		return
	}

	c.startOnce.Do(func() {
		c.startRWLoop()
	})
}

func (c *Connection) State() ConnState {
	if atomic.LoadUint32(&c.closed) == 1 {
		return ConnClosed
	}
	if atomic.LoadUint32(&c.connected) == 1 {
		return ConnActive
	}
	return ConnInit
}

func (c *Connection) startRWLoop() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("%s connection read loop crashed, local addr: %s, remote addr: %s, err: %+v",
					c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("%s connection close due to read loop crashed failed, local addr: %s, remote addr: %s, err: %+v",
							c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
					}
				}()
				_ = c.Close(NoFlush, LocalClose)
			}()
		}()
		c.startReadLoop()
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("%s connection write loop crashed, local addr: %s, remote addr: %s, err: %+v",
					c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("%s connection close due to write loop crashed failed, local addr: %s, remote addr: %s, err: %+v",
							c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
					}
				}()
				_ = c.Close(NoFlush, LocalClose)
			}()
		}()
		c.startWriteLoop()
	}()
}

func (c *Connection) startReadLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		case <-c.readEnabledChan:
		default:
			if c.readEnabled {
				err := c.doReadIO()
				if err != nil {
					if te, ok := err.(net.Error); ok && te.Timeout() {
						if c.protocol == "tcp" && c.readBuffer != nil && c.readBuffer.Len() == 0 && c.readBuffer.Cap() > DefaultBufferReadCapacity {
							c.readBuffer.Free()
							c.readBuffer.Alloc(DefaultBufferReadCapacity)
						}
						continue
					}

					// normal close or health check, modify log level
					if c.lastBytesSizeRead == 0 || err == io.EOF {
						logger.Debugf("%s connection error on read, local addr: %s, remote addr: %s, err: %s",
							c.protocol, c.localAddr.String(), c.remoteAddr.String(), err.Error())
					} else {
						logger.Errorf("%s connection error on read, local addr: %s, remote addr: %s, err: %s",
							c.protocol, c.localAddr.String(), c.remoteAddr.String(), err.Error())
					}

					if err == io.EOF {
						_ = c.Close(NoFlush, RemoteClose)
					} else {
						_ = c.Close(NoFlush, OnReadErrClose)
					}
					return
				}
			} else {
				select {
				case <-c.readEnabledChan:
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
	}
}

func (c *Connection) setReadDeadline() {
	switch c.protocol {
	case "udp":
		_ = c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	case "tcp":
		_ = c.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	}
}

func (c *Connection) startWriteLoop() {
	var err error
	for {
		select {
		case <-c.stopChan:
			return
		case buf, ok := <-c.writeBufferChan:
			if !ok {
				return
			}
			c.appendBuffer(buf)
		OUTER:
			for i := 0; i < 10; i++ {
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

			c.setWriteDeadline()
			_, err = c.doWrite()
		}

		if err != nil {
			if err == iobufferpool.EOF {
				logger.Debugf("%s connection error on write, local addr: %s, remote addr: %s, err: %+v",
					c.protocol, c.localAddr.String(), c.remoteAddr.String(), err)
				_ = c.Close(NoFlush, LocalClose)
			} else {
				logger.Errorf("%s connection error on write, local addr: %s, remote addr: %s, err: %+v",
					c.protocol, c.localAddr.String(), c.remoteAddr.String(), err)
			}

			if te, ok := err.(net.Error); ok && te.Timeout() {
				_ = c.Close(NoFlush, OnWriteTimeout)
			}
			if c.protocol == "udp" && strings.Contains(err.Error(), "connection refused") {
				_ = c.Close(NoFlush, RemoteClose)
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

func (c *Connection) Close(ccType CloseType, eventType Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s connection has closed, local addr: %s, remote addr: %s, err: %+v",
				c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
			err = ErrConnectionHasClosed
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
	if c.conn == nil || reflect.ValueOf(c.conn).IsNil() {
		return nil
	}

	// close tcp conn read first
	if tconn, ok := c.conn.(*net.TCPConn); ok {
		logger.Errorf("%s connection close read, local addr: %s, remote addr: %s",
			c.protocol, c.localAddr.String(), c.remoteAddr.String())
		_ = tconn.CloseRead()
	}

	// close conn recv, then notify read/write loop to exit
	close(c.stopChan)
	_ = c.conn.Close()

	logger.Errorf("%s connection closed, local addr: %s, remote addr: %s",
		c.protocol, c.localAddr.String(), c.remoteAddr.String())
	return nil
}

func (c *Connection) doReadIO() (err error) {
	if c.readBuffer == nil {
		switch c.protocol {
		case "udp":
			// A UDP socket will Read up to the size of the receiving buffer and will discard the rest
			c.readBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
		default: // unix or tcp
			c.readBuffer = iobufferpool.GetIoBuffer(DefaultBufferReadCapacity)
		}
	}

	var bytesRead int64
	c.setReadDeadline()
	bytesRead, err = c.readBuffer.ReadOnce(c.conn)

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
		logger.Errorf("%s connection ReadOnce maybe always return (0, nil) and causes dead loop, local addr: %s, remote addr: %s",
			c.protocol, c.localAddr.String(), c.remoteAddr.String())
	}

	if !c.readEnabled {
		return
	}

	if bufLen := c.readBuffer.Len(); bufLen == 0 {
		return
	} else {
		buf := c.readBuffer.Clone()
		c.readBuffer.Drain(bufLen)
		c.onRead(buf)

		if int64(bufLen) != c.lastBytesSizeRead {
			c.lastBytesSizeRead = int64(bufLen)
		}
	}
	return
}

func (c *Connection) doWrite() (int64, error) {
	bytesSent, err := c.doWriteIO()
	if err != nil && atomic.LoadUint32(&c.closed) == 1 {
		return 0, nil
	}

	if bytesSent > 0 {
		bytesBufSize := int64(c.writeBufLen())
		if int64(c.writeBufLen()) != c.lastWriteSizeWrite {
			c.lastWriteSizeWrite = bytesBufSize
		}
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
	switch c.protocol {
	case "tcp":
		bytesSent, err = buffers.WriteTo(c.conn)
	case "udp":
		addr := c.remoteAddr.(*net.UDPAddr)
		n := 0
		bytesSent = 0
		for _, buf := range c.ioBuffers {
			if c.conn.RemoteAddr() == nil {
				n, err = c.conn.(*net.UDPConn).WriteToUDP(buf.Bytes(), addr)
			} else {
				n, err = c.conn.Write(buf.Bytes())
			}
			if err != nil {
				break
			}
			bytesSent += int64(n)
		}
	}

	if err != nil {
		return bytesSent, err
	}
	for i, buf := range c.ioBuffers {
		c.ioBuffers[i] = nil
		c.writeBuffers[i] = nil
		if buf.EOF() {
			err = iobufferpool.EOF
		}
		if e := iobufferpool.PutIoBuffer(buf); e != nil {
			logger.Errorf("%s connection PutIoBuffer error, local addr: %s, remote addr: %s, err: %+v",
				c.protocol, c.localAddr.String(), c.remoteAddr.String(), err)
		}
	}
	c.ioBuffers = c.ioBuffers[:0]
	c.writeBuffers = c.writeBuffers[:0]
	return
}

// Write receive other connection data
func (c *Connection) Write(buffers ...iobufferpool.IoBuffer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s connection has closed, local addr: %s, remote addr: %s, err: %+v",
				c.protocol, c.localAddr.String(), c.remoteAddr.String(), r)
			err = ErrConnectionHasClosed
		}
	}()

	bufs := c.onWrite(buffers)
	if bufs == nil {
		return
	}

	select {
	case c.writeBufferChan <- &buffers:
		return
	default:
	}

	// fail after 60s
	t := timerpool.Get(60 * time.Second)
	select {
	case c.writeBufferChan <- &buffers:
	case <-t.C:
		err = ErrWriteBufferChanTimeout
	}
	timerpool.Put(t)
	return
}

func (c *Connection) setWriteDeadline() {
	switch c.protocol {
	case "udp":
		_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	case "tcp":
		_ = c.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	}
}

// UpstreamConnection wrap connection to upstream
type UpstreamConnection struct {
	Connection
	connectTimeout time.Duration
	connectOnce    sync.Once
}

func NewUpstreamConnection(connectTimeout time.Duration, remoteAddr net.Addr, stopChan chan struct{},
	onRead func(buffer iobufferpool.IoBuffer), onWrite func(src []iobufferpool.IoBuffer) []iobufferpool.IoBuffer) *UpstreamConnection {
	res := &UpstreamConnection{
		Connection: Connection{
			connected:  1,
			remoteAddr: remoteAddr,

			readEnabled:     true,
			readEnabledChan: make(chan bool, 1),
			writeBufferChan: make(chan *[]iobufferpool.IoBuffer, 8),

			mu:       sync.Mutex{},
			stopChan: stopChan,
			onRead:   onRead,
			onWrite:  onWrite,
		},
		connectTimeout: connectTimeout,
	}
	if res.remoteAddr != nil {
		res.Connection.protocol = res.remoteAddr.Network()
	}
	return res
}

func (u *UpstreamConnection) connect() (event Event, err error) {
	timeout := u.connectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	addr := u.remoteAddr
	if addr == nil {
		return ConnectFailed, errors.New("upstream addr is nil")
	}
	u.conn, err = net.DialTimeout(u.protocol, addr.String(), timeout)
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
	event = Connected
	u.localAddr = u.conn.LocalAddr()
	return
}

func (u *UpstreamConnection) Connect() (err error) {
	u.connectOnce.Do(func() {
		var event Event
		event, err = u.connect()
		if err == nil {
			u.Start()
		}
		logger.Debugf("connect upstream, upstream addr: %s, event: %+v, err: %+v", u.remoteAddr, event, err)
		if event != Connected {
			close(u.stopChan) // if upstream connection failed, close client connection
		}
	})
	return
}
