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

package objectpool

import (
	"context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestValidate(t *testing.T) {
	assertions := assert.New(t)

	spec := &Spec{
		InitSize: 3,
		MaxSize:  2,
	}

	err := spec.Validate()
	assertions.Nil(err)
	assertions.True(spec.MaxSize == spec.InitSize)

	spec.MaxSize = 0
	assertions.NoError(spec.Validate())

	spec.InitSize, spec.MaxSize = 0, 0
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

func TestNewSimplePool(t *testing.T) {
	init, max := 2, 4
	pool := New(init, max, func() (PoolObject, error) {
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

func benchmarkWithIPoolObjectNumAndGoroutineNum(iPoolObjNum, goRoutineNum int, fake PoolObject, b *testing.B) {
	pool := New(iPoolObjNum/2, iPoolObjNum, func() (PoolObject, error) {
		return fake, nil
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
					getAndPut(pool)
				}
			}
		}()
	}
	startedWait.Wait()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getAndPut(pool)
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
