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
)

// CloseType represent connection close type
type CloseType string

//Connection close types
const (
	// FlushWrite means write buffer to underlying io then close connection
	FlushWrite CloseType = "FlushWrite"
	// NoFlush means close connection without flushing buffer
	NoFlush CloseType = "NoFlush"
)

// ConnectionEvent type
type ConnectionEvent string

const (
	RemoteClose    ConnectionEvent = "RemoteClose"
	LocalClose     ConnectionEvent = "LocalClose"
	OnReadErrClose ConnectionEvent = "OnReadErrClose"
	Connected      ConnectionEvent = "ConnectedFlag"
	ConnectTimeout ConnectionEvent = "ConnectTimeout"
	ConnectFailed  ConnectionEvent = "ConnectFailed"
	OnWriteTimeout ConnectionEvent = "OnWriteTimeout"
)

var (
	ErrConnectionHasClosed    = errors.New("connection has closed")
	ErrWriteBufferChanTimeout = errors.New("writeBufferChan has timeout")
)

// ConnState status
type ConnState int

// Connection statuses
const (
	ConnInit ConnState = iota
	ConnActive
	ConnClosed
)
