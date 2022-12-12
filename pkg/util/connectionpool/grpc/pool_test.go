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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestPool(t *testing.T) {
	target := "localhost:8081"
	gomonkey.ApplyMethod(&grpc.ClientConn{}, "GetState", func(_ *grpc.ClientConn) connectivity.State {
		return connectivity.Ready
	})
	gomonkey.ApplyMethod(&grpc.ClientConn{}, "Target", func(_ *grpc.ClientConn) string {
		return target
	})
	gomonkey.ApplyMethod(&grpc.ClientConn{}, "Close", func(_ *grpc.ClientConn) error {
		return nil
	})
	gomonkey.ApplyFunc(grpc.DialContext, func(ctx context.Context, target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
		return &grpc.ClientConn{}, nil
	})
	assertions := assert.New(t)
	connectionsPerHost := 3
	pool, err := New(&Spec{
		BorrowTimeout:      1000 * time.Millisecond,
		ConnectTimeout:     1000 * time.Millisecond,
		ConnectionsPerHost: connectionsPerHost,
		DialOptions:        []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithCodec(GetCodecInstance())},
	})
	assertions.NoError(err)
	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			wrapper, err := pool.Get(target)
			assertions.NoError(err)
			connection, ok := wrapper.(*ClientConn)
			assertions.True(ok)
			assertions.NoError(connection.ReturnPool())
			wg.Done()
		}()
	}
	wg.Wait()
	sgt, exit := pool.segment.Load(target)
	assertions.True(exit)
	assertions.True(connectionsPerHost <= len(sgt.(*segment).clients))

	wrapper, _ := pool.Get(target)
	_ = wrapper.(*ClientConn).ReturnPool()
	err = wrapper.(*ClientConn).ReturnPool()
	assertions.Equal(err, ErrAlreadyClosed)
	pool.Close()
	assertions.True(pool.IsClosed())
	_, err = pool.Get(target)
	assertions.Equal(ErrClosed, err)
	assertions.Equal(ErrClosed, wrapper.(*ClientConn).ReturnPool())
}
