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

package grpcproxy

import (
	"context"
	"github.com/megaease/easegress/pkg/util/objectpool"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"sync"
	"testing"
)

func multiGetAndPut(pool *MultiPool, ctx context.Context) {
	iPoolObject, _ := pool.Get(ctx)
	if iPoolObject != nil {
		pool.Put(ctx, iPoolObject)
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
	pool := NewMultiWithSpec(&objectpool.Spec{InitSize: 1, MaxSize: 2, New: func(ctx context.Context) (objectpool.PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	}})
	separateCtx := SetSeparatedKey(context.Background(), "123")
	oldObj1, err := pool.Get(separateCtx)
	pool.Put(separateCtx, oldObj1)
	as := assert.New(t)
	as.NoError(err)

	separateCtx = SetSeparatedKey(context.Background(), "123")
	oldObj2, err := pool.Get(separateCtx)
	pool.Put(separateCtx, oldObj2)
	as.NoError(err)
	as.True(oldObj2 == oldObj1)

	ctx := SetSeparatedKey(context.Background(), "234")
	newObj, err := pool.Get(ctx)
	pool.Put(ctx, newObj)
	as.NoError(err)

	as.True(newObj != oldObj1)
	count := 0
	pool.pools.Range(func(key, value any) bool {
		count++
		return true
	})
	as.Equal(2, count)

	as.Panics(func() {
		pool.Get(context.Background())
	})

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
		New: func(ctx context.Context) (objectpool.PoolObject, error) {
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
					ctx := SetSeparatedKey(context.Background(), keys[rand.Intn(keyNum)])
					multiGetAndPut(pool, ctx)
				}
			}
		}()
	}
	startedWait.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := SetSeparatedKey(context.Background(), keys[rand.Intn(keyNum)])
		multiGetAndPut(pool, ctx)
	}
	b.StopTimer()
	close(ch)
}

func BenchmarkMultiGoroutine2TimesIPoolObject(b *testing.B) {
	benchmarkMultiWithIPoolObjectNumAndGoroutineNum(2, 4, &fakeNormalPoolObject{random: true}, b)
}
