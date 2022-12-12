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

package grpc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/connectionpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// ErrorParams is the error when create pool with invalid params
	ErrorParams = errors.New("invalid params to create pool")
	// ErrClosed is the error when the client pool is closed
	ErrClosed = errors.New("grpc pool: client pool is closed")
	// ErrTimeout is the error when the client pool timed out
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	// ErrAlreadyClosed is the error when the client conn was already closed
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	// ErrFullPool is the error when the pool is already full
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
	// ErrFactory is the error when factory occur exception
	ErrFactory = errors.New("grpc pool: connection factory occur exception")

	defaultDialOpts = []grpc.DialOption{grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(GetCodecInstance())}
	_ connectionpool.Pool = (*Pool)(nil)
)

const (
	closed uint32 = iota
	normal
)

type (
	// Spec describe Pool
	Spec struct {
		BorrowTimeout      time.Duration
		ConnectTimeout     time.Duration
		ConnectionsPerHost int
		DialOptions        []grpc.DialOption
	}

	// Pool is the grpc client pool
	Pool struct {
		segment  sync.Map
		factory  connectionpool.CreateConnFactory
		spec     *Spec
		mu       sync.RWMutex
		isClosed bool
	}

	segment struct {
		count   int32
		clients chan *ClientConn
		pool    *Pool
	}

	// ClientConn is the wrapper for a grpc client conn
	ClientConn struct {
		*grpc.ClientConn
		segment  *segment
		returned uint32
	}
)

func newPool(factory connectionpool.CreateConnFactory, spec *Spec) (*Pool, error) {
	// grpc-go office suggest use one Connection and multiple stream.
	// and think of to balance pressure of tcp packet parsing on multiple cpus.
	// refer to https://github.com/grpc/grpc-go/issues/3005, so default we design
	// init conn num is 2. we adopt producer-consumer model, for each addr,
	// first call Get() and we create init num conn to pool , then if consumer
	// consume too fast, we would create new conn until reach Spec.MaxConnectionsPerHost
	// if reach max, we use conn with random policy
	if spec.ConnectionsPerHost <= 0 {
		spec.ConnectionsPerHost = 2
	}
	if spec.BorrowTimeout == 0 {
		spec.BorrowTimeout = 500 * time.Millisecond
	}
	if spec.ConnectTimeout == 0 {
		spec.ConnectTimeout = 200 * time.Millisecond
	}

	if len(spec.DialOptions) == 0 {
		spec.DialOptions = defaultDialOpts
	}

	p := &Pool{
		factory: factory,
		spec:    spec,
	}
	return p, nil
}

// New creates a new clients pool with the given initial and maximum capacity.
// Returns an error if the initial clients could not be created
func New(spec *Spec) (*Pool, error) {
	return newPool(nil, spec)
}

// NewWithFactory creates a new clients pool with the given initial and maximum
// capacity. The context parameter would be passed to the factory method during initialization.
// Returns an error if the initial clients could not be created.
func NewWithFactory(factory connectionpool.CreateConnFactory, spec *Spec) (*Pool, error) {
	if factory == nil {
		return nil, ErrorParams
	}
	return newPool(factory, spec)
}

// Close empties the pool calling Close on all its clients.
// You can call Close while there are outstanding clients.
// The pool channel is then closed, and Get will not be allowed anymore
func (p *Pool) Close() {
	p.mu.RLock()
	if p.isClosed {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()
	p.mu.Lock()
	defer func() {
		p.isClosed = true
		p.mu.Unlock()
	}()

	p.segment.Range(func(key, value any) bool {
		close(value.(*segment).clients)
		for c := range value.(*segment).clients {
			if c.ClientConn != nil {
				c.ClientConn.Close()
			}
		}
		return true
	})

}

// IsClosed returns true if the client pool is closed.
func (p *Pool) IsClosed() bool {
	return p == nil || p.isClosed
}

// Get will return the next available client. If capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will wait till the next client becomes available or a timeout.
// A timeout of 0 is an indefinite wait
func (p *Pool) Get(addr string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.IsClosed() {
		return nil, ErrClosed
	}
	timeout, cancelFunc := context.WithTimeout(context.Background(), p.spec.BorrowTimeout)
	defer cancelFunc()
	var smt *segment
	if value, ok := p.segment.Load(addr); !ok {
		value, _ = p.segment.LoadOrStore(addr, &segment{
			clients: make(chan *ClientConn, p.spec.ConnectionsPerHost),
			pool:    p,
			count:   0,
		})
		smt = value.(*segment)
	} else {
		smt = value.(*segment)
	}

	for {
		select {
		case wrapper := <-smt.clients:
			if wrapper.isAvailable() {
				atomic.StoreUint32(&wrapper.returned, normal)
				return wrapper, nil
			}
			// discard
			atomic.AddInt32(&smt.count, -1)
		case <-timeout.Done():
			return nil, ErrTimeout // it would better returns ctx.Err()
		default:
			cur := atomic.LoadInt32(&smt.count)
			if cur >= int32(smt.Capacity()) || !atomic.CompareAndSwapInt32(&smt.count, cur, cur+1) {
				continue
			}
			return func() (wrapper *ClientConn, err error) {
				defer func() {
					if wrapper == nil {
						atomic.AddInt32(&smt.count, -1)
					}
				}()
				if p.factory == nil {
					return p.createConnectionByDefault(timeout, addr, smt)
				} else {
					return p.createConnectionByFactory(timeout, addr, smt)
				}
			}()
		}
	}
}

func (p *Pool) createConnectionByDefault(ctx context.Context, addr string, smt *segment) (wrapper *ClientConn, err error) {
	cc, err := grpc.DialContext(ctx, addr, p.spec.DialOptions...)
	if err != nil {
		return nil, err
	}
	wrapper = &ClientConn{
		segment:    smt,
		ClientConn: cc,
		returned:   normal,
	}
	return wrapper, nil
}

func (p *Pool) createConnectionByFactory(ctx context.Context, addr string, smt *segment) (wrapper *ClientConn, err error) {
	cc, err := p.factory(ctx, addr)
	if err != nil {
		return nil, ErrFactory
	}
	if connection, ok := cc.(*grpc.ClientConn); ok {
		wrapper = &ClientConn{
			segment:    smt,
			ClientConn: connection,
			returned:   normal,
		}
		return wrapper, nil
	} else {
		return nil, ErrFactory
	}
}

// ReturnPool returns a ClientConn to the pool. It is safe to call multiple time,
// but will return an error after first time
func (c *ClientConn) ReturnPool() error {
	if c == nil || c.ClientConn == nil {
		return nil
	}
	c.segment.pool.mu.RLock()
	poolClosed := c.segment.pool.IsClosed()
	c.segment.pool.mu.RUnlock()
	if poolClosed {
		return ErrClosed
	}

	if atomic.LoadUint32(&c.returned) == closed || !atomic.CompareAndSwapUint32(&c.returned, normal, closed) {
		return ErrAlreadyClosed
	}

	if !c.isAvailable() {
		c.ClientConn.Close()
		// help gc
		c.ClientConn = nil
		atomic.AddInt32(&c.segment.count, -1)
	} else {
		select {
		case c.segment.clients <- c:
			// All good
		default:
			return ErrFullPool
		}
	}
	return nil
}

func (c *ClientConn) isAvailable() bool {
	logger.Debugf("connection target %s state is %v", c.Target(), c.GetState())
	return c.GetState() == connectivity.Ready || c.GetState() == connectivity.Idle
}

// Capacity returns the capacity
func (s *segment) Capacity() int {
	s.pool.mu.RLock()
	defer s.pool.mu.RUnlock()
	if s.pool.IsClosed() {
		return 0
	}
	return cap(s.clients)
}
