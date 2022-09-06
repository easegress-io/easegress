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
	"github.com/agiledragon/gomonkey/v2"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestConcurrentGet(t *testing.T) {
	assertions := assert.New(t)
	pool := NewPool(&Spec{
		BorrowTimeout:   1 * time.Second,
		ConnectTimeout:  1 * time.Second,
		MaxConnsPerHost: 4,
		InitConnNum:     2,
	})
	gomonkey.ApplyMethod(&grpc.ClientConn{}, "GetState", func(_ *grpc.ClientConn) connectivity.State {
		return connectivity.Ready
	})
	gomonkey.ApplyFunc(grpc.DialContext, func(ctx context.Context, target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
		return &grpc.ClientConn{}, nil
	})

	conn, err := pool.Get("localhost:8848")
	assertions.NoError(err)
	assertions.NotNil(conn)
	for j := 0; j < 20; j++ {
		count := 200
		threads := &sync.WaitGroup{}
		threads.Add(count)
		concurrent := &sync.WaitGroup{}
		concurrent.Add(count)
		errCount := uint32(0)
		runnable := func() {
			defer func() {
				threads.Done()
			}()
			concurrent.Wait()
			conn, err := pool.Get("localhost:8848")
			if err != nil {
				atomic.AddUint32(&errCount, 1)
				return
			}
			pool.ReleaseConn(conn)
		}
		for i := 0; i < count; i++ {
			go runnable()
			concurrent.Done()
		}
		threads.Wait()
		assertions.Equal(uint32(0), errCount)
	}
}
