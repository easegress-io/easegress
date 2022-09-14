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
	"fmt"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/connectionpool"

	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type (
	// segment lock and server for the one address
	segmentConn struct {
		*sync.Mutex
		connCh    chan *Connection
		connMap   map[*grpc.ClientConn]interface{}
		waitedNum int32
		addr      string
	}

	// Pool grpc-go office suggest use one Connection and multiple stream.
	// and think of to balance pressure of tcp packet parsing on multiple cpus.
	// refer to https://github.com/grpc/grpc-go/issues/3005, so default we design
	// init conn num is 2. we adopt producer-consumer model, for each addr,
	// first call Get() and we create init num conn to pool , then if consumer
	// consume too fast, we would create new conn until reach Spec.MaxConnsPerHost
	// if reach max, we use conn with random policy
	Pool struct {
		spec       *Spec
		connsMutex *sync.Mutex
		conns      map[string]*segmentConn
		isDone     bool
		factory    connectionpool.CreateConnFactory
	}
	// Connection wrapper grpc.ClientConn
	Connection struct {
		*grpc.ClientConn
		segment  *segmentConn
		needBack bool
	}

	// Spec describe Pool
	Spec struct {
		BorrowTimeout   time.Duration
		ConnectTimeout  time.Duration
		MaxConnsPerHost uint16
		InitConnNum     uint16
		DialOptions     []grpc.DialOption
	}
)

const (
	defaultInitNum             = 2
	defaultBorrowTimeout       = 500 * time.Millisecond
	defaultConnectTimeout      = 200 * time.Millisecond
	defaultMaxIdleConnsPerHost = 4
)

var (
	defaultDialOpts = []grpc.DialOption{grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(GetCodecInstance())}
	_ connectionpool.Pool = (*Pool)(nil)
)

func NewPool(spec *Spec) *Pool {
	if spec.InitConnNum == 0 {
		spec.InitConnNum = defaultInitNum
	}
	if spec.BorrowTimeout == 0 {
		spec.BorrowTimeout = defaultBorrowTimeout
	}
	if spec.ConnectTimeout == 0 {
		spec.ConnectTimeout = defaultConnectTimeout
	}
	if spec.MaxConnsPerHost == 0 {
		spec.MaxConnsPerHost = defaultMaxIdleConnsPerHost
	}
	if spec.DialOptions == nil {
		spec.DialOptions = defaultDialOpts
	}

	logger.Infof("created grpc connection pool with spec %+v", spec)
	return &Pool{
		spec:       spec,
		connsMutex: &sync.Mutex{},
		conns:      map[string]*segmentConn{},
	}
}

func NewPoolWithFactory(spec *Spec, factory connectionpool.CreateConnFactory) *Pool {
	if factory == nil {
		panic("the factory shouldn't be nil")
	}
	pool := NewPool(spec)
	pool.factory = factory
	return pool
}

func (s *segmentConn) buildConnectionWithChannel(factory connectionpool.CreateConnFactory, spec *Spec, errCh chan<- error) {
	s.Lock()
	defer s.Unlock()
	waited := int(atomic.LoadInt32(&s.waitedNum))
	if waited <= len(s.connCh) || len(s.connCh) == cap(s.connCh) {
		return
	}
	// it means all conn in used, so we pick random conn
	if len(s.connMap) == int(spec.MaxConnsPerHost) {
		for c := range s.connMap {
			if c.GetState() == connectivity.Shutdown {
				delete(s.connMap, c)
				continue
			}
			s.connCh <- &Connection{ClientConn: c, segment: s, needBack: false}
			return
		}
	}

	for i := 0; i < int(spec.InitConnNum); i++ {
		rawConn, err := buildConnection(s.addr, factory, spec)
		if err != nil {
			errCh <- status.Errorf(codes.Internal, "couldn't create connection for addr %s, cause: %s", s.addr, err.Error())
			logger.Infof("not create connection for addr %s, cause: %s", s.addr, err.Error())
			return
		}
		c := &Connection{
			segment:    s,
			ClientConn: rawConn,
			needBack:   true,
		}
		s.connCh <- c
		s.connMap[rawConn] = nil
		// other goroutine return conn
		if len(s.connMap) >= int(spec.MaxConnsPerHost) {
			break
		}
	}
}

func (m *Pool) Get(addr string) (interface{}, error) {
	timeout, cancelFunc := context.WithTimeout(context.Background(), m.spec.BorrowTimeout)
	defer cancelFunc()
	if m.conns[addr] == nil {
		m.connsMutex.Lock()
		if m.conns[addr] == nil {
			m.conns[addr] = &segmentConn{
				connCh:    make(chan *Connection, m.spec.MaxConnsPerHost),
				connMap:   make(map[*grpc.ClientConn]interface{}, m.spec.MaxConnsPerHost),
				Mutex:     &sync.Mutex{},
				waitedNum: 0,
				addr:      addr,
			}
		}
		m.connsMutex.Unlock()
	}
	atomic.AddInt32(&m.conns[addr].waitedNum, 1)
	defer atomic.AddInt32(&m.conns[addr].waitedNum, -1)
	errCh := make(chan error, 1)
	go m.conns[addr].buildConnectionWithChannel(m.factory, m.spec, errCh)
	for {
		select {
		case <-timeout.Done():
			return nil, timeout.Err()
		case c := <-m.conns[addr].connCh:
			if c.isAvailable() {
				return c, nil
			}
			c.destroyConn()
			go m.conns[addr].buildConnectionWithChannel(m.factory, m.spec, errCh)
		case err := <-errCh:
			return nil, err
		}
	}
}

// ReleaseConn grpc share connection, so no need release
func (m *Pool) ReleaseConn(i interface{}) {
	if conn, ok := i.(*Connection); ok {
		if !conn.isAvailable() {
			conn.destroyConn()
		} else {
			conn.backChannel()
		}
	}
}

func (m *Pool) Close() {
	if m.isDone {
		return
	}
	// help gc and we wouldn't close the connections to avoid affecting in-transit requests and go channel
	m.conns = nil
	m.isDone = true
}

func buildConnection(addr string, factory connectionpool.CreateConnFactory, spec *Spec) (*grpc.ClientConn, error) {
	var dial interface{}
	var err error
	if factory != nil {
		dial, err = factory()
	} else {
		timeout, cancelFunc := context.WithTimeout(context.Background(), spec.ConnectTimeout)
		defer cancelFunc()
		dial, err = grpc.DialContext(timeout, addr, spec.DialOptions...)
	}

	if err != nil {
		logger.Warnf("couldn't create available connection for addr %s, cause %s", addr, err.Error())
		return nil, err
	}

	if v, ok := dial.(*grpc.ClientConn); !ok {
		msg := fmt.Sprintf("create connection %T isn't instance of grpc.clientConn", dial)
		logger.Warnf(msg)
		return nil, errors.New(msg)
	} else {
		return v, nil
	}
}

// in the actual test process, we found that if the server suddenly goes offline, the tcp connection between easegress and server
// will be destroy, but the status of grpc.ClientConn will not change to Shutdown, it switch between Idle and TransientFailure,
// and the server will go online again after some minutes, these ClientConn can be used normally and it's status is Idle or Ready.
// based on above-mentioned reason and grpc's official comments, tentatively, TransientFailure conn will not be regarded as abnormal and removed from pool,
// but this is only experimental and may be changed in the future based on feedback from actual use results
func (c *Connection) isAvailable() bool {
	logger.Debugf("connection target %s state is %v", c.Target(), c.GetState())
	return c.GetState() != connectivity.Shutdown
}

func (c *Connection) backChannel() {
	if !c.needBack {
		return
	}
	c.segment.Lock()
	defer c.segment.Unlock()
	if len(c.segment.connCh) < cap(c.segment.connCh) {
		c.segment.connCh <- c
		return
	}
	for len(c.segment.connCh) >= cap(c.segment.connCh) {
		select {
		case cc := <-c.segment.connCh:
			if cc.needBack {
				c.segment.connCh <- cc
			}
		default:
		}
	}
	c.segment.connCh <- c
}

func (c *Connection) destroyConn() {
	c.segment.Lock()
	defer c.segment.Unlock()
	delete(c.segment.connMap, c.ClientConn)
}
