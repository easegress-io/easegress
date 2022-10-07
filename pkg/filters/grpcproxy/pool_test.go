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

package grpcprxoy

import (
	"context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWithoutConnPool(t *testing.T) {
	Convey("test proxy without pool", t, func() {
		assertions := assert.New(t)

		yaml := `
kind: GRPCProxy
useConnectionPool: false
initConnNum: 1
port: 8849
pools:
  - loadBalance:
      policy: forward
      forwardKey: forwardKey
    serviceName: "easegress"
name: grpcforwardproxy
`
		proxy := newTestProxy(yaml, assertions)
		//pool := NewServerPool(proxy, &ServerPoolSpec{LoadBalance: &LoadBalanceSpec{Policy: "forward"}}, "test")
		c := &grpc.ClientConn{}
		gomonkey.ApplyMethod(reflect.TypeOf(c), "GetState", func(_ *grpc.ClientConn) connectivity.State {
			return connectivity.Ready
		})

		gomonkey.ApplyFunc(grpc.DialContext, func(ctx context.Context, target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
			return c, nil
		})
		wait := &sync.WaitGroup{}
		finish := &sync.WaitGroup{}
		loop := 100
		wait.Add(loop)
		finish.Add(loop)
		count := uint32(0)
		dial := func() {
			wait.Wait()
			if _, err := proxy.mainPool.dial("1", "1"); err != nil {
				atomic.AddUint32(&count, 1)
			}
			finish.Done()
		}
		for i := 0; i < loop; i++ {
			go dial()
			wait.Done()
		}
		finish.Wait()
		assertions.True(0 == count)

		assertions.True(getSyncMapSize(&proxy.conns) == 1)
	})

	Convey("test proxy without pool", t, func() {
		assertions := assert.New(t)

		yaml := `
kind: GRPCProxy
useConnectionPool: false
initConnNum: 1
port: 8849
pools:
  - loadBalance:
      policy: forward
    serviceName: "easegress"
name: grpcforwardproxy
`
		proxy := newTestProxy(yaml, assertions)
		//pool := NewServerPool(proxy, &ServerPoolSpec{LoadBalance: &LoadBalanceSpec{Policy: "forward"}}, "test")
		c := &grpc.ClientConn{}
		gomonkey.ApplyMethod(c, "GetState", func(_ *grpc.ClientConn) connectivity.State {
			return connectivity.Ready
		})

		gomonkey.ApplyFunc(grpc.DialContext, func(ctx context.Context, target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
			return c, nil
		})
		wait := &sync.WaitGroup{}
		finish := &sync.WaitGroup{}
		loop := 100
		wait.Add(loop)
		finish.Add(loop)
		count := uint32(0)
		dial := func() {
			wait.Wait()
			if _, err := proxy.mainPool.dial("1", "1"); err != nil {
				atomic.AddUint32(&count, 1)
			}
			finish.Done()
		}
		for i := 0; i < loop; i++ {
			go dial()
			wait.Done()
		}
		finish.Wait()
		assertions.True(0 == count)
		assertions.True(getSyncMapSize(&proxy.conns) == 1)
	})

}

func getSyncMapSize(m *sync.Map) int {
	eleNum := 0
	m.Range(func(key, value any) bool {
		eleNum++
		return true
	})
	return eleNum
}
