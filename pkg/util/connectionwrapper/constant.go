package connectionwrapper

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
