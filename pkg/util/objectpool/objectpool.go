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

// Package objectpool provides Pool of interface PoolObject
package objectpool

import (
	"context"
	"fmt"
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
)

// PoolObject is an interface that about definition of object that managed by pool
type PoolObject interface {
	Destroy()          // destroy the object
	HealthCheck() bool // check the object is health or not
}

type (
	// Pool manage the PoolObject
	Pool struct {
		spec  *Spec
		size  int             // current size
		store chan PoolObject // store the object
		cond  *sync.Cond      // when conditions are met, it wakes all goroutines waiting on sync.Cond
	}

	// Spec Pool's spec
	Spec struct {
		InitSize     int            // initial size
		MaxSize      int            // max size
		Init         CreateObjectFn // create init size object, it must return a health object or err
		CheckWhenGet bool           // whether to health check when get PoolObject
		CheckWhenPut bool           // whether to health check when put PoolObject
	}
	// CreateObjectFn create new object, it must return a health object or err
	CreateObjectFn func() (PoolObject, error)
)

// New returns a new pool
func New(initSize, maxSize int, init func() (PoolObject, error)) *Pool {
	return NewWithSpec(&Spec{
		InitSize:     initSize,
		MaxSize:      maxSize,
		Init:         init,
		CheckWhenGet: true,
		CheckWhenPut: true,
	})
}

// NewWithSpec returns a new pool
func NewWithSpec(spec *Spec) *Pool {
	p := &Pool{
		spec:  spec,
		store: make(chan PoolObject, spec.MaxSize),
		cond:  sync.NewCond(&sync.Mutex{}),
	}

	for i := 0; i < p.spec.InitSize && p.spec.Init != nil; i++ {
		obj, err := p.spec.Init()
		if err != nil {
			logger.Errorf("create pool object failed: %v", err)
			continue
		}
		p.size++
		p.store <- obj
	}

	return p
}

// Validate validate
func (s *Spec) Validate() error {
	if s.InitSize > s.MaxSize {
		s.MaxSize = s.InitSize
	}
	if s.MaxSize <= 0 {
		return fmt.Errorf("pool max size must be positive")
	}
	if s.InitSize < 0 {
		return fmt.Errorf("pool init size must greate than or equals 0")
	}
	if (s.Init == nil && s.InitSize != 0) || (s.Init != nil && s.InitSize == 0) {
		return fmt.Errorf("func init and init size must be either nil or not nil at the same time")
	}
	return nil
}

// The fast path, try get an object from the pool directly
func (p *Pool) fastGet() PoolObject {
	select {
	case obj := <-p.store:
		return obj
	default:
		return nil
	}
}

// The slow path, we need to wait for an object or create a new one.
func (p *Pool) slowGet(ctx context.Context, new CreateObjectFn) (PoolObject, error) {
	// we need to watch ctx.Done in another goroutine, so that we can stop
	// the slow path when the context is done.
	// we also need to stop the watch when the slow path is done.
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		select {
		case <-ctx.Done():
			p.cond.Broadcast()
		case <-stop:
		}
	}()

	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case obj := <-p.store:
			return obj, nil

		default:
		}

		// try creating a new object
		if p.size < p.spec.MaxSize {
			if obj, err := new(); err == nil {
				p.size++
				return obj, nil
			}
		}

		// the pool reaches its max size and there is no object available
		p.cond.Wait()
	}
}

// Get returns an object from the pool,
//
// if there's an available object, it will return it directly;
// if there's no free object, it will create a one if the pool is not full;
// if the pool is full, it will block until an object is returned to the pool.
func (p *Pool) Get(ctx context.Context, new CreateObjectFn) (PoolObject, error) {
	for {
		obj := p.fastGet()
		if obj == nil {
			var err error
			obj, err = p.slowGet(ctx, new)
			if err != nil {
				return nil, err
			}
		}

		if !p.spec.CheckWhenGet || obj.HealthCheck() {
			return obj, nil
		}

		p.putUnhealthyObject(obj)
	}
}

func (p *Pool) putUnhealthyObject(obj PoolObject) {
	p.cond.L.Lock()
	p.size--
	p.cond.L.Unlock()

	p.cond.Signal()
	obj.Destroy()
}

// Put return the object to the pool
func (p *Pool) Put(obj PoolObject) {
	if obj == nil {
		panic("pool: put nil object")
	}

	if p.spec.CheckWhenPut && !obj.HealthCheck() {
		p.putUnhealthyObject(obj)
		return
	}

	p.store <- obj
	p.cond.Signal()
}

// Close closes the pool and clean all the objects
func (p *Pool) Close() {
	close(p.store)
	for obj := range p.store {
		obj.Destroy()
	}
}
