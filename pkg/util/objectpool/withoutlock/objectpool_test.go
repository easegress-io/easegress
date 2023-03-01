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

package withoutlock

import (
	"context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"runtime"
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

func TestValidate(t *testing.T) {
	assertions := assert.New(t)

	spec := &Spec{
		initSize: 3,
		maxSize:  2,
	}

	err := spec.Validate()
	assertions.Nil(err)
	assertions.True(spec.maxSize == spec.initSize)

	spec.maxSize = 0
	assertions.NoError(spec.Validate())

	spec.initSize, spec.maxSize = 0, 0
	assertions.Error(spec.Validate())
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

// fakeAlmostUnHealthPoolObject 80% unhealthy poolObject
type fakeAlmostUnHealthPoolObject struct {
}

func (f *fakeAlmostUnHealthPoolObject) Destroy() {

}

func (f *fakeAlmostUnHealthPoolObject) HealthCheck() bool {
	random := rand.Intn(10)
	return random >= 8
}

type fakeUnHealthPoolObject struct {
}

func (f *fakeUnHealthPoolObject) Destroy() {

}

func (f *fakeUnHealthPoolObject) HealthCheck() bool {
	return false
}

func TestNewSimplePool(t *testing.T) {
	init, max := 2, 4
	pool := NewSimplePool(int32(init), int32(max), func() (IPoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})

	as := assert.New(t)
	as.Equal(len(pool.store), init)
	as.Equal(cap(pool.store), max)
}

func getAndPut(pool *Pool) {
	iPoolObject, _ := pool.Get(context.Background())
	if iPoolObject != nil {
		pool.Put(iPoolObject)
	}
}

func benchmarkWithIPoolObjectNumAndGoroutineNum(iPoolObjNum, goRoutineNum int, fake IPoolObject, b *testing.B) {
	pool := NewSimplePool(int32(iPoolObjNum/2), int32(iPoolObjNum), func() (IPoolObject, error) {
		return fake, nil
	})
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					getAndPut(pool)
				}
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getAndPut(pool)
	}
	close(ch)
}

func BenchmarkWithoutRace(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(1, 1, &fakeNormalPoolObject{random: true}, b)
}

func BenchmarkIPoolObjectEqualsGoroutine(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(4, 4, &fakeNormalPoolObject{random: true}, b)
}

func BenchmarkGoroutine2TimesIPoolObject(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(2, 4, &fakeNormalPoolObject{random: true}, b)
}

func BenchmarkGoroutine4TimesIPoolObject(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(2, 8, &fakeNormalPoolObject{random: true}, b)
}

func BenchmarkGoroutine2TimesIPoolObjectWithAlmostUnHealthIPoolObject(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(2, 4, &fakeAlmostUnHealthPoolObject{}, b)
}

func BenchmarkGoroutine4TimesIPoolObjectWithAlmostUnHealthIPoolObject(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(2, 8, &fakeAlmostUnHealthPoolObject{}, b)
}

type fakePool struct {
	*Pool
}

func (f *fakePool) Get(ctx context.Context) (IPoolObject, error) {
	backoff := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case poolObject := <-f.store:
			if f.checkWhenGet && !poolObject.HealthCheck() {
				f.destroyIPoolObject(poolObject)
				if poolObject = f.createIPoolObject(); poolObject != nil {
					return poolObject, nil
				}
				continue
			}
			return poolObject, nil
		default:
			if iPoolObject := f.createIPoolObject(); iPoolObject != nil {
				return iPoolObject, nil
			}
			for i := 0; i < backoff; i++ {
				runtime.Gosched()
			}
			if backoff < maxBackoff {
				backoff <<= 1
			}
		}
	}
}

func (f *fakePool) createIPoolObject() IPoolObject {
	if atomic.AddInt32(&f.size, 1) > f.maxSize {
		atomic.AddInt32(&f.size, -1)
		return nil
	}

	if iPoolObject, err := f.new(); err == nil {
		return iPoolObject
	}

	atomic.AddInt32(&f.size, -1)
	return nil
}

func newFakePool(initSize, maxSize int32, new func() (IPoolObject, error)) *fakePool {
	return newFakePoolWithSpec(Spec{
		initSize:     initSize,
		maxSize:      maxSize,
		new:          new,
		checkWhenGet: true,
		checkWhenPut: true,
	})
}

func newFakePoolWithSpec(spec Spec) *fakePool {
	p := &fakePool{}
	p.Pool = NewPoolWithSpec(spec)
	if err := p.init(); err != nil {
		logger.Errorf("new pool failed %v", err)
		return nil
	}
	return p
}

func BenchmarkGoroutine32TimesRawPoolWithUnHealthIPoolObject(b *testing.B) {
	iPoolObjNum := 2
	goRoutineNum := 64
	fake := &fakeUnHealthPoolObject{}
	pool := NewSimplePool(int32(iPoolObjNum/2), int32(iPoolObjNum), func() (IPoolObject, error) {
		return fake, nil
	})
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					iPoolObject, _ := pool.Get(context.Background())
					if iPoolObject != nil {
						pool.Put(iPoolObject)
					}
				}
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iPoolObject, _ := pool.Get(context.Background())
		if iPoolObject != nil {
			pool.Put(iPoolObject)
		}
	}
	close(ch)
}

func BenchmarkGoroutine32TimesRawPoolWithUnHealthIPoolObjectTimeout(b *testing.B) {
	iPoolObjNum := 2
	goRoutineNum := 64
	fake := &fakeUnHealthPoolObject{}
	pool := NewSimplePool(int32(iPoolObjNum/2), int32(iPoolObjNum), func() (IPoolObject, error) {
		return fake, nil
	})
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					timeout, cancelFunc := context.WithTimeout(context.Background(), 200*time.Millisecond)
					iPoolObject, _ := pool.Get(timeout)
					if iPoolObject != nil {
						pool.Put(iPoolObject)
					}
					cancelFunc()
				}
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timeout, cancelFunc := context.WithTimeout(context.Background(), 200*time.Millisecond)
		iPoolObject, _ := pool.Get(timeout)
		if iPoolObject != nil {
			pool.Put(iPoolObject)
		}
		cancelFunc()
	}
	close(ch)
}

func BenchmarkGoroutine32TimesFakePoolWithUnHealthIPoolObject(b *testing.B) {
	iPoolObjNum := 2
	goRoutineNum := 64
	fake := &fakeUnHealthPoolObject{}
	pool := newFakePool(int32(iPoolObjNum/2), int32(iPoolObjNum), func() (IPoolObject, error) {
		return fake, nil
	})
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					iPoolObject, _ := pool.Get(context.Background())
					if iPoolObject != nil {
						pool.Put(iPoolObject)
					}
				}
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iPoolObject, _ := pool.Get(context.Background())
		if iPoolObject != nil {
			pool.Put(iPoolObject)
		}
	}
	close(ch)
}

func BenchmarkGoroutine32TimesFakePoolWithUnHealthIPoolObjectTimeout(b *testing.B) {
	iPoolObjNum := 2
	goRoutineNum := 64
	fake := &fakeUnHealthPoolObject{}
	pool := newFakePool(int32(iPoolObjNum/2), int32(iPoolObjNum), func() (IPoolObject, error) {
		return fake, nil
	})
	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(goRoutineNum - 1)
	for i := 0; i < goRoutineNum-1; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					timeout, cancelFunc := context.WithTimeout(context.Background(), 200*time.Millisecond)
					iPoolObject, _ := pool.Get(timeout)
					if iPoolObject != nil {
						pool.Put(iPoolObject)
					}
					cancelFunc()
				}
			}
		}()
	}
	wg.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timeout, cancelFunc := context.WithTimeout(context.Background(), 200*time.Millisecond)
		iPoolObject, _ := pool.Get(timeout)
		if iPoolObject != nil {
			pool.Put(iPoolObject)
		}
		cancelFunc()
	}
	close(ch)
}
