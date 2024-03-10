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

package grpcproxy

import (
	"context"
	"math/rand"
	"sync"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"

	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/util/objectpool"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSPSValidate(t *testing.T) {
	at := assert.New(t)
	sps := &ServerPoolSpec{}
	at.Error(sps.Validate())

	sps.ServiceName = "demo"
	at.NoError(sps.Validate())

	sps.Servers = []*Server{
		{Weight: 10},
		{Weight: 0},
	}
	at.Error(sps.Validate())
	sps.Servers[1].Weight = 1
	at.NoError(sps.Validate())
}

func TestCreateLB(t *testing.T) {
	s := &ServerPool{}
	lb := s.CreateLoadBalancer(&LoadBalanceSpec{Policy: LoadBalancePolicyForward}, nil)
	at := assert.New(t)
	at.IsType(&forwardLoadBalancer{}, lb)

}

func multiGetAndPut(pool *MultiPool, key string, ctx context.Context) {
	iPoolObject, _ := pool.Get(key, ctx, func() (objectpool.PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})
	if iPoolObject != nil {
		pool.Put(key, iPoolObject)
	}
}

type fakeNormalPoolObject struct {
	random bool
	health bool
}

func (f *fakeNormalPoolObject) Destroy() {

}

func (f *fakeNormalPoolObject) HealthCheck() bool {
	if !f.random {
		return f.health
	}
	random := rand.Intn(10)
	return random != 8
}

func TestNewSimpleMultiPool(t *testing.T) {
	pool := NewMultiWithSpec(&objectpool.Spec{MaxSize: 2})
	oldObj1, err := pool.Get("123", context.Background(), func() (objectpool.PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})
	pool.Put("123", oldObj1)
	as := assert.New(t)
	as.NoError(err)

	oldObj2, err := pool.Get("123", context.Background(), func() (objectpool.PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})
	pool.Put("123", oldObj2)
	as.NoError(err)
	as.True(oldObj2 == oldObj1)

	newObj, err := pool.Get("234", context.Background(), func() (objectpool.PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})
	pool.Put("234", newObj)
	as.NoError(err)

	as.True(newObj != oldObj1)
	count := 0
	pool.pools.Range(func(key, value any) bool {
		count++
		return true
	})
	as.Equal(2, count)

}

func benchmarkMultiWithIPoolObjectNumAndGoroutineNum(iPoolObjNum, goRoutineNum int, fake objectpool.PoolObject, b *testing.B) {
	var keys []string
	keyNum := 200
	for i := 0; i < keyNum; i++ {
		keys = append(keys, string(rune(i)))
	}
	pool := NewMultiWithSpec(&objectpool.Spec{
		InitSize: iPoolObjNum / 2,
		MaxSize:  iPoolObjNum,
		Init: func() (objectpool.PoolObject, error) {
			return fake, nil
		},
		CheckWhenGet: true,
		CheckWhenPut: true,
	})
	ch := make(chan struct{})
	startedWait := sync.WaitGroup{}
	startedWait.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			done := false
			for {
				select {
				case <-ch:
					return
				default:
					if !done {
						startedWait.Done()
						done = true
					}

					multiGetAndPut(pool, keys[rand.Intn(keyNum)], context.Background())
				}
			}
		}()
	}
	startedWait.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		multiGetAndPut(pool, keys[rand.Intn(keyNum)], context.Background())
	}
	b.StopTimer()
	close(ch)
}

func BenchmarkMultiGoroutine2TimesIPoolObject(b *testing.B) {
	benchmarkMultiWithIPoolObjectNumAndGoroutineNum(2, 4, &fakeNormalPoolObject{random: true}, b)
}

func TestServerPoolError(t *testing.T) {
	spe := serverPoolError{
		result: "ok",
		status: status.New(codes.OK, "ok"),
	}

	assert.Equal(t, "ok", spe.Result())
	assert.Equal(t, int(codes.OK), spe.Code())
	assert.Equal(t, "server pool error, status code=rpc error: code = OK desc = ok, result=ok", spe.Error())
}

func TestServerPoolSpecValidate(t *testing.T) {
	sps := &ServerPoolSpec{}
	assert.Error(t, sps.Validate())

	sps.Servers = []*proxies.Server{
		{
			URL:  "http://192.168.1.1:80",
			Tags: []string{"a1"},
		},
		{
			URL:  "http://192.168.1.2:80",
			Tags: []string{"a1"},
		},
	}
	assert.NoError(t, sps.Validate())

	sps.Servers[0].Weight = 1
	assert.Error(t, sps.Validate())
}

func TestGetTarget(t *testing.T) {
	at := assert.New(t)

	s := `
kind: GRPCProxy
pools:
 - loadBalance:
     policy: roundRobin
   servers:
    - url: http://192.168.1.1:80
    - url: http://192.168.1.2:80
   serviceName: easegress
maxIdleConnsPerHost: 2
connectTimeout: 100ms
borrowTimeout: 100ms
name: grpcforwardproxy
`
	proxy := newTestProxy(s, at)

	server := proxy.mainPool.LoadBalancer().ChooseServer(nil)

	at.NotEqual("", proxy.mainPool.getTarget(server.URL))

	proxy.Close()

	s = `
kind: GRPCProxy
pools:
 - loadBalance:
     policy: forward
     forwardKey: targetAddress
   serviceName: easegress
maxIdleConnsPerHost: 2
connectTimeout: 100ms
borrowTimeout: 100ms
name: grpcforwardproxy
`
	proxy = newTestProxy(s, at)
	request := grpcprot.NewRequestWithContext(context.Background())
	request.Header().Add("targetAddress", "192.168.1.1:8080")

	at.Equal("192.168.1.1:8080", proxy.mainPool.getTarget(proxy.mainPool.LoadBalancer().ChooseServer(request).URL))

	request.Header().Set("targetAddress", "192.168.1.1")
	at.Equal("", proxy.mainPool.getTarget(proxy.mainPool.LoadBalancer().ChooseServer(request).URL))
}
