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
	"github.com/casbin/casbin/v2/log"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/iobufferpool"
)

type Connection struct {
	conn       net.Conn
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

	onRead  func(buffer iobufferpool.IoBuffer)
	onWrite func(src iobufferpool.IoBuffer) iobufferpool.IoBuffer
}

func New(conn net.Conn, stopChan chan struct{}, remoteAddr net.Addr) *Connection {
	res := &Connection{
		conn:      conn,
		protocol:  conn.LocalAddr().Network(),
		localAddr: conn.LocalAddr(),

		mu:              sync.Mutex{},
		stopChan:        stopChan,
		readEnabledChan: make(chan bool, 1),
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
		log.DefaultLogger.Errorf("[network] ReadOnce maybe always return (0, nil) and causes dead loop, Connection = %d, Local Address = %+v, Remote Address = %+v",
			c.id, c.rawConnection.LocalAddr(), c.RemoteAddr())
	}

	c.onRead(bytesRead)
	return
}

func (c *Connection) doWriteIO() {

}

func (c *Connection) Write(buffers ...iobufferpool.IoBuffer) interface{} {

}
