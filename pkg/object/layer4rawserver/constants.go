package layer4rawserver

import (
	"errors"
	"time"
)

var (
	ErrConnectionHasClosed    = errors.New("connection has closed")
	ErrWriteTryLockTimeout    = errors.New("write trylock has timeout")
	ErrWriteBufferChanTimeout = errors.New("writeBufferChan has timeout")
)

// Default connection arguments
const (
	DefaultBufferReadCapacity = 1 << 7

	DefaultConnReadTimeout  = 15 * time.Second
	DefaultConnWriteTimeout = 15 * time.Second
	DefaultConnTryTimeout   = 60 * time.Second
	DefaultIdleTimeout      = 90 * time.Second
	DefaultUDPIdleTimeout   = 5 * time.Second
	DefaultUDPReadTimeout   = 1 * time.Second
	ConnReadTimeout         = 15 * time.Second
)

// ConnState Connection status
type ConnState int

// Connection statuses
const (
	ConnInit ConnState = iota
	ConnActive
	ConnClosed
)

type ListenerState int

// listener state
// ListenerActivated means listener is activated, an activated listener can be started or stopped
// ListenerRunning means listener is running, start a running listener will be ignored.
// ListenerStopped means listener is stopped, start a stopped listener without restart flag will be ignored.
const (
	ListenerActivated ListenerState = iota
	ListenerRunning
	ListenerStopped
)
