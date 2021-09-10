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
	"time"
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

// Event type
type Event string

// ConnectionEvent types
const (
	RemoteClose     Event = "RemoteClose"
	LocalClose      Event = "LocalClose"
	OnReadErrClose  Event = "OnReadErrClose"
	OnWriteErrClose Event = "OnWriteErrClose"
	OnConnect       Event = "OnConnect"
	Connected       Event = "ConnectedFlag"
	ConnectTimeout  Event = "ConnectTimeout"
	ConnectFailed   Event = "ConnectFailed"
	OnReadTimeout   Event = "OnReadTimeout"
	OnWriteTimeout  Event = "OnWriteTimeout"
)

var (
	ErrConnectionHasClosed    = errors.New("connection has closed")
	ErrWriteTryLockTimeout    = errors.New("write trylock has timeout")
	ErrWriteBufferChanTimeout = errors.New("writeBufferChan has timeout")
)

// Network related const
const (
	DefaultBufferReadCapacity = 1 << 7

	NetBufferDefaultSize     = 0
	NetBufferDefaultCapacity = 1 << 4

	DefaultConnectTimeout = 10 * time.Second
)
