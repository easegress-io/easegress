/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package limitlistener provides a Listener that accepts at most n simultaneous.
package limitlistener

import (
	"context"
	"net"
	"sync"

	sem2 "github.com/megaease/easegress/v2/pkg/util/sem"
)

// NewLimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func NewLimitListener(l net.Listener, n uint32) *LimitListener {
	ctx, cancel := context.WithCancel(context.Background())

	return &LimitListener{
		Listener: l,
		sem:      sem2.NewSem(n),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// LimitListener is the Listener to limit connections.
type LimitListener struct {
	net.Listener
	sem       *sem2.Semaphore
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once // ensures the done chan is only closed once
}

// acquire acquires the limiting semaphore. Returns true if successfully
// accquired, false if the listener is closed and the semaphore is not
// acquired.
func (l *LimitListener) acquire() bool {
	return l.sem.AcquireWithContext(l.ctx) == nil
}

func (l *LimitListener) release() {
	l.sem.Release()
}

// Accept accepts one connection.
func (l *LimitListener) Accept() (net.Conn, error) {
	acquired := l.acquire()
	if err := l.ctx.Err(); err != nil {
		if acquired {
			l.release()
		}
		return nil, err
	}

	c, err := l.Listener.Accept()
	if err != nil {
		l.release()
		return nil, err
	}
	return &limitListenerConn{Conn: c, release: l.release}, nil
}

// SetMaxConnection sets max connection.
func (l *LimitListener) SetMaxConnection(n uint32) {
	l.sem.SetMaxCount(int64(n))
}

// Close closes LimitListener.
func (l *LimitListener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(l.cancel)
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
