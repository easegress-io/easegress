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

package objectpool

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestNew(t *testing.T) {
	assert := assert.New(t)

	p := New(1, 2, func() (PoolObject, error) { return nil, nil })
	assert.Equal(1, p.spec.InitSize)
	assert.Equal(2, p.spec.MaxSize)
	assert.Equal(true, p.spec.CheckWhenGet)
	assert.Equal(true, p.spec.CheckWhenPut)
	obj, err := p.spec.Init()
	assert.Nil(err)
	assert.Nil(obj)

	p = NewWithSpec(&Spec{
		InitSize: 2,
		MaxSize:  3,
		Init: func() (PoolObject, error) {
			return nil, nil
		},
		CheckWhenGet: false,
		CheckWhenPut: false,
	})
	assert.Equal(2, p.spec.InitSize)
	assert.Equal(3, p.spec.MaxSize)
	assert.Equal(false, p.spec.CheckWhenGet)
	assert.Equal(false, p.spec.CheckWhenPut)
	obj, err = p.spec.Init()
	assert.Nil(err)
	assert.Nil(obj)

	p = New(1, 2, func() (PoolObject, error) { return nil, errors.New("error") })
	obj, err = p.spec.Init()
	assert.NotNil(err)
	assert.Nil(obj)
}

func TestValidate(t *testing.T) {
	assert := assert.New(t)

	createSpec := func(initSize, maxSize int, init CreateObjectFn) *Spec {
		return &Spec{
			InitSize: initSize,
			MaxSize:  maxSize,
			Init:     init,
		}
	}

	createObjFn := func() (PoolObject, error) {
		return nil, nil
	}

	testCases := []struct {
		spec *Spec
		err  bool
	}{
		{spec: createSpec(1, 2, createObjFn), err: false},
		{spec: createSpec(3, 2, createObjFn), err: false},
		{spec: createSpec(-1, -2, createObjFn), err: true},
		{spec: createSpec(0, 1, nil), err: false},
		{spec: createSpec(-1, 2, createObjFn), err: true},
		{spec: createSpec(1, 2, nil), err: true},
	}

	for _, tc := range testCases {
		if tc.err {
			assert.NotNil(tc.spec.Validate())
		} else {
			assert.Nil(tc.spec.Validate())
		}
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

// fakeAlmostUnHealthPoolObject 80% unhealthy poolObject
type fakeAlmostUnHealthPoolObject struct {
}

func (f *fakeAlmostUnHealthPoolObject) Destroy() {

}

func (f *fakeAlmostUnHealthPoolObject) HealthCheck() bool {
	random := rand.Intn(10)
	return random >= 8
}

func TestNewSimplePool(t *testing.T) {
	init, max := 2, 4
	pool := New(init, max, func() (PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	})

	as := assert.New(t)
	as.Equal(len(pool.store), init)
	as.Equal(cap(pool.store), max)
}

func getAndPut(pool *Pool, new CreateObjectFn) {
	iPoolObject, _ := pool.Get(context.Background(), new)
	if iPoolObject != nil {
		pool.Put(iPoolObject)
	}
}

func benchmarkWithIPoolObjectNumAndGoroutineNum(iPoolObjNum, goRoutineNum int, fake PoolObject, b *testing.B) {
	fn := func() (PoolObject, error) {
		return fake, nil
	}
	pool := New(iPoolObjNum/2, iPoolObjNum, fn)
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
					getAndPut(pool, fn)
				}
			}
		}()
	}
	startedWait.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getAndPut(pool, fn)
	}
	b.StopTimer()
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

// fakeUnHealthPoolObject 100% unhealthy poolObject
type fakeUnHealthPoolObject struct {
}

func (f *fakeUnHealthPoolObject) Destroy() {

}

func (f *fakeUnHealthPoolObject) HealthCheck() bool {
	return false
}

func BenchmarkGoroutine2TimesIPoolObjectWithUnhHealthyPool(b *testing.B) {
	benchmarkWithIPoolObjectNumAndGoroutineNum(2, 4, &fakeUnHealthPoolObject{}, b)
}

func TestPool(t *testing.T) {
	assert := assert.New(t)

	createObjectFn := func() (PoolObject, error) {
		return &fakeNormalPoolObject{random: false, health: true}, nil
	}

	pool := New(1, 2, createObjectFn)

	// fast get
	obj, err := pool.Get(context.Background(), createObjectFn)
	assert.Nil(err)
	assert.NotNil(obj)
	assert.True(obj.HealthCheck())

	// slow get
	obj, err = pool.Get(context.Background(), createObjectFn)
	assert.Nil(err)
	assert.NotNil(obj)
	assert.True(obj.HealthCheck())

	// slow get, wait for timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	obj, err = pool.Get(ctx, createObjectFn)
	assert.Nil(obj)
	assert.NotNil(err)
	assert.Equal(ctx.Err(), err)

	// slow get, wait for put
	putObj := &fakeNormalPoolObject{random: false, health: true}
	go func() {
		time.Sleep(time.Second)
		pool.Put(putObj)
	}()
	obj, err = pool.Get(context.Background(), createObjectFn)
	assert.Nil(err)
	assert.NotNil(obj)
	assert.Equal(putObj, obj)

	// put unhealthy object
	putObj = &fakeNormalPoolObject{random: false, health: false}
	pool.Put(putObj)
	assert.Equal(0, len(pool.store))

	// put nil
	assert.Panics(func() {
		pool.Put(nil)
	})
	obj, err = createObjectFn()
	assert.Nil(err)
	pool.Put(obj)

	pool.Close()
}
