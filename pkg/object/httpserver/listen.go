package httpserver

import (
	"net"
	"sync"

	sem2 "github.com/megaease/easegateway/pkg/util/sem"
)

// NewLimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func NewLimitListener(l net.Listener, n uint32) *LimitListener {
	return &LimitListener{
		Listener: l,
		sem:      sem2.NewSem(n),
		done:     make(chan struct{}),
	}
}

// LimitListener is the Listener to limit connections.
type LimitListener struct {
	net.Listener
	sem       *sem2.Semaphore
	closeOnce sync.Once     // ensures the done chan is only closed once
	done      chan struct{} // no values sent; closed when Close is called
}

// acquire acquires the limiting semaphore. Returns true if successfully
// accquired, false if the listener is closed and the semaphore is not
// acquired.
func (l *LimitListener) acquire() bool {
	select {
	case <-l.done:
		return false
	case <-l.sem.AcquireRaw():
		return true
	}
}
func (l *LimitListener) release() { l.sem.Release() }

// Accept accepts one conneciton.
func (l *LimitListener) Accept() (net.Conn, error) {
	acquired := l.acquire()
	// If the semaphore isn't acquired because the listener was closed, expect
	// that this call to accept won't block, but immediately return an error.
	c, err := l.Listener.Accept()
	if err != nil {
		if acquired {
			l.release()
		}
		return nil, err
	}
	return &limitListenerConn{Conn: c, release: l.release}, nil
}

// SetMaxConnection sets max connection.
func (l *LimitListener) SetMaxConnection(n uint32) {
	l.sem.SetMaxCount(n)
}

// Close closes LimitListener.
func (l *LimitListener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(func() { close(l.done) })
	return err
}

type limitListenerConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (l *limitListenerConn) Close() error {
	err := l.Conn.Close()
	l.releaseOnce.Do(l.release)
	return err
}
