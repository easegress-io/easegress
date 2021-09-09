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

package connectionwrapper

import (
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
	"github.com/megaease/easegress/pkg/util/timerpool"
)

// ConnectionCloseType represent connection close type
type ConnectionCloseType string

//Connection close types
const (
	// FlushWrite means write buffer to underlying io then close connection
	FlushWrite ConnectionCloseType = "FlushWrite"
	// NoFlush means close connection without flushing buffer
	NoFlush ConnectionCloseType = "NoFlush"
)

// ConnectionEvent type
type ConnectionEvent string

// ConnectionEvent types
const (
	RemoteClose     ConnectionEvent = "RemoteClose"
	LocalClose      ConnectionEvent = "LocalClose"
	OnReadErrClose  ConnectionEvent = "OnReadErrClose"
	OnWriteErrClose ConnectionEvent = "OnWriteErrClose"
	OnConnect       ConnectionEvent = "OnConnect"
	Connected       ConnectionEvent = "ConnectedFlag"
	ConnectTimeout  ConnectionEvent = "ConnectTimeout"
	ConnectFailed   ConnectionEvent = "ConnectFailed"
	OnReadTimeout   ConnectionEvent = "OnReadTimeout"
	OnWriteTimeout  ConnectionEvent = "OnWriteTimeout"
)

type Connection struct {
	net.Conn

	closed    uint32
	connected uint32
	startOnce sync.Once

	// readLoop/writeLoop goroutine fields:
	internalStopChan chan struct{}
	readEnabled      bool
	readEnabledChan  chan bool // if we need to reload read filters, it's better to stop read data before reload filters

	lastBytesSizeRead  int64
	lastWriteSizeWrite int64

	curWriteBufferData []iobufferpool.IoBuffer
	readBuffer         iobufferpool.IoBuffer
	writeBuffers       net.Buffers
	ioBuffers          []iobufferpool.IoBuffer
	writeBufferChan    chan *[]iobufferpool.IoBuffer
}

func New(conn net.Conn) *Connection {
	return &Connection{
		Conn: conn,
	}
}

func (c *Connection) StartRWLoop(ctx context.Layer4Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("%s connection read loop crashed, local addr: %s, remote addr: %s, err: %+v",
					ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("%s connection close due to read loop crashed failed, local addr: %s, remote addr: %s, err: %+v",
							ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
					}
				}()
				_ = c.Close(NoFlush, LocalClose, ctx)
			}()
		}()
		c.startReadLoop(ctx)
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("%s connection write loop crashed, local addr: %s, remote addr: %s, err: %+v",
					ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("%s connection close due to write loop crashed failed, local addr: %s, remote addr: %s, err: %+v",
							ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
					}
				}()
				_ = c.Close(NoFlush, LocalClose, ctx)
			}()
		}()
		c.startWriteLoop(ctx)
	}()
}

func (c *Connection) startReadLoop(ctx context.Layer4Context) {
	for {
		select {
		case <-c.internalStopChan:
			return
		case <-c.readEnabledChan:
		default:
			if c.readEnabled {
				err := c.doRead(ctx)
				if err != nil {
					if te, ok := err.(net.Error); ok && te.Timeout() {
						if ctx.Protocol() == "tcp" && c.readBuffer != nil &&
							c.readBuffer.Len() == 0 && c.readBuffer.Cap() > DefaultBufferReadCapacity {
							c.readBuffer.Free()
							c.readBuffer.Alloc(DefaultBufferReadCapacity)
						}
						continue
					}

					if c.lastBytesSizeRead == 0 || err == io.EOF {
						logger.Debugf("%s connection write loop closed, local addr: %s, remote addr: %s",
							ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
					} else {
						logger.Errorf("%s connection write loop closed, local addr: %s, remote addr: %s",
							ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
					}

					if err == io.EOF {
						_ = c.Close(NoFlush, RemoteClose, ctx)
					} else {
						_ = c.Close(NoFlush, OnReadErrClose, ctx)
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

func (c *Connection) startWriteLoop(ctx context.Layer4Context) {
	defer func() {
		close(c.writeBufferChan)
	}()

	var err error
	for {
		select {
		case <-c.internalStopChan:
			return
		case buf, ok := <-c.writeBufferChan:
			if !ok {
				return
			}
			c.appendBuffer(buf)

		QUIT:
			for i := 0; i < 10; i++ {
				select {
				case buf, ok := <-c.writeBufferChan:
					if !ok {
						return
					}
					c.appendBuffer(buf)
				default:
					break QUIT
				}

				c.setWriteDeadline(ctx)
				_, err = c.doWrite(ctx)
			}
		}

		if err != nil {

			if err == iobufferpool.EOF {
				logger.Debugf("%s connection write loop occur error, local addr: %s, remote addr: %s",
					ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
				c.Close(NoFlush, LocalClose, ctx)
			} else {
				logger.Errorf("%s connection write loop occur error, local addr: %s, remote addr: %s",
					ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
			}

			if te, ok := err.(net.Error); ok && te.Timeout() {
				c.Close(NoFlush, OnWriteTimeout, ctx)
			}

			if ctx.Protocol() == "udp" && strings.Contains(err.Error(), "connection refused") {
				c.Close(NoFlush, RemoteClose, ctx)
			}
			//other write errs not close connection, because readbuffer may have unread data, wait for readloop close connection,

			return
		}
	}
}

func (c *Connection) appendBuffer(buffers *[]iobufferpool.IoBuffer) {
	if buffers == nil {
		return
	}
	for _, buf := range *buffers {
		if buf == nil {
			continue
		}
		c.ioBuffers = append(c.ioBuffers, buf)
		c.writeBuffers = append(c.writeBuffers, buf.Bytes())
	}
}

func (c *Connection) doRead(ctx context.Layer4Context) (err error) {
	if c.readBuffer == nil {
		switch ctx.Protocol() {
		case "udp":
			// A UDP socket will Read up to the size of the receiving buffer and will discard the rest
			c.readBuffer = iobufferpool.GetIoBuffer(iobufferpool.UdpPacketMaxSize)
		default:
			c.readBuffer = iobufferpool.GetIoBuffer(DefaultBufferReadCapacity)
		}
	}

	var bytesRead int64
	c.setReadDeadline(ctx)
	bytesRead, err = c.readBuffer.ReadOnce(c.Conn)

	if err != nil {
		if atomic.LoadUint32(&c.closed) == 1 {
			return err
		}

		if te, ok := err.(net.Error); ok && te.Timeout() {
			// TODO add timeout handle(such as send keepalive msg to active connection)

			if bytesRead == 0 {
				return err
			}
		} else if err != io.EOF {
			return err
		}
	}

	//todo: ReadOnce maybe always return (0, nil) and causes dead loop (hack)
	if bytesRead == 0 && err == nil {
		err = io.EOF
		logger.Errorf("%s connection read maybe always return (0, nil), local addr: %s, remote addr: %s",
			ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
	}
	c.lastBytesSizeRead = int64(c.readBuffer.Len())
	return
}

// Write send recv data(batch mode) to upstream
func (c *Connection) Write(ctx context.Layer4Context, buffers ...iobufferpool.IoBuffer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s connection has closed, local addr: %s, remote addr: %s, err: %+v",
				ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
			err = ErrConnectionHasClosed
		}
	}()

	// TODO get filters from layer4 pipeline, transform buffers via filters

	select {
	case c.writeBufferChan <- &buffers:
		return
	default:
	}

	t := timerpool.Get(DefaultConnTryTimeout)
	select {
	case c.writeBufferChan <- &buffers:
	case <-t.C:
		err = ErrWriteBufferChanTimeout
	}
	timerpool.Put(t)
	return
}

func (c *Connection) setWriteDeadline(ctx context.Layer4Context) {
	args := ctx.ConnectionArgs()
	if args.ProxyWriteTimeout > 0 {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(time.Duration(args.ProxyWriteTimeout) * time.Millisecond))
	} else {
		switch ctx.Protocol() {
		case "udp":
			_ = c.Conn.SetWriteDeadline(time.Now().Add(DefaultUDPIdleTimeout))
		case "tcp":
			_ = c.Conn.SetWriteDeadline(time.Now().Add(DefaultConnWriteTimeout))
		}
	}
}

func (c *Connection) setReadDeadline(ctx context.Layer4Context) {
	args := ctx.ConnectionArgs()
	if args.ProxyWriteTimeout > 0 {
		_ = c.Conn.SetReadDeadline(time.Now().Add(time.Duration(args.ProxyReadTimeout) * time.Millisecond))
	} else {
		switch ctx.Protocol() {
		case "udp":
			_ = c.Conn.SetReadDeadline(time.Now().Add(DefaultUDPReadTimeout))
		case "tcp":
			_ = c.Conn.SetReadDeadline(time.Now().Add(ConnReadTimeout))
		}
	}
}

// Close handle connection close event
func (c *Connection) Close(ccType ConnectionCloseType, eventType ConnectionEvent, ctx context.Layer4Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s connection has closed, local addr: %s, remote addr: %s, err: %+v",
				ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), r)
			err = ErrConnectionHasClosed
		}
	}()

	if ccType == FlushWrite {
		_ = c.Write(ctx, iobufferpool.NewIoBufferEOF())
		return nil
	}

	if !atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
		return nil
	}

	// connection failed in client mode
	if c.Conn == nil || reflect.ValueOf(c.Conn).IsNil() {
		return nil
	}

	// close tcp conn read first
	if tconn, ok := c.Conn.(*net.TCPConn); ok {
		logger.Errorf("%s connection close read, local addr: %s, remote addr: %s",
			ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
		_ = tconn.CloseRead()
	}

	// close conn recv, then notify read/write loop to exit
	close(c.internalStopChan)
	_ = c.Conn.Close()

	logger.Errorf("%s connection closed, local addr: %s, remote addr: %s",
		ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String())
	return nil
}

func (c *Connection) writeBufLen() (bufLen int) {
	for _, buf := range c.writeBuffers {
		bufLen += len(buf)
	}
	return
}

func (c *Connection) doWrite(ctx context.Layer4Context) (interface{}, error) {
	bytesSent, err := c.doWriteIO(ctx)
	if err != nil && atomic.LoadUint32(&c.closed) == 1 {
		return 0, nil
	}

	c.lastWriteSizeWrite = int64(c.writeBufLen())
	return bytesSent, err
}

//
func (c *Connection) doWriteIO(ctx context.Layer4Context) (bytesSent int64, err error) {
	buffers := c.writeBuffers
	switch ctx.Protocol() {
	case "udp":
		addr := ctx.RemoteAddr().(*net.UDPAddr)
		n := 0
		bytesSent = 0
		for _, buf := range c.ioBuffers {
			if c.Conn.RemoteAddr() == nil {
				n, err = c.Conn.(*net.UDPConn).WriteToUDP(buf.Bytes(), addr)
			} else {
				n, err = c.Conn.Write(buf.Bytes())
			}
			if err != nil {
				break
			}
			bytesSent += int64(n)
		}
	case "tcp":
		bytesSent, err = buffers.WriteTo(c.Conn)
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
			logger.Errorf("%s connection give io buffer failed, local addr: %s, remote addr: %s, err: %s",
				ctx.Protocol(), ctx.LocalAddr().String(), ctx.RemoteAddr().String(), err.Error())
		}
	}
	c.ioBuffers = c.ioBuffers[:0]
	c.writeBuffers = c.writeBuffers[:0]
	return
}

func (c *Connection) SetNoDelay(enable bool) {
	if c.Conn != nil {
		if tconn, ok := c.Conn.(*net.TCPConn); ok {
			_ = tconn.SetNoDelay(enable)
		}
	}
}

func (c *Connection) ReadEnabled() bool {
	return c.readEnabled
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

func (c *Connection) GetReadBuffer() iobufferpool.IoBuffer {
	return c.readBuffer
}
