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

// Package objectpool provides Pool of interface PoolObject
package objectpool

import (
	"context"
	"fmt"
	"github.com/megaease/easegress/pkg/logger"
	"sync"
	"sync/atomic"
)

// PoolObject is an interface that about definition of object that managed by pool
type PoolObject interface {
	Destroy()          // destroy the object
	HealthCheck() bool // check the object is health or not
}

type (
	// Pool manage the PoolObject
	Pool struct {
		initSize     int32                      // initial size
		maxSize      int32                      // max size
		size         int32                      // current size
		new          func() (PoolObject, error) // create a new object, it must return a health object or err
		store        chan PoolObject            // store the object
		cond         *sync.Cond                 // when conditions are met, it wakes all goroutines waiting on sync.Cond
		checkWhenGet bool                       // whether to health check when get PoolObject
		checkWhenPut bool                       // whether to health check when put PoolObject
	}

	// Spec Pool's spec
	Spec struct {
		InitSize     int32                      // initial size
		MaxSize      int32                      // max size
		New          func() (PoolObject, error) // create a new object
		CheckWhenGet bool                       // whether to health check when get PoolObject
		CheckWhenPut bool                       // whether to health check when put PoolObject
	}
)

// New returns a new pool
func New(initSize, maxSize int32, new func() (PoolObject, error)) *Pool {
	return NewWithSpec(Spec{
		InitSize:     initSize,
		MaxSize:      maxSize,
		New:          new,
		CheckWhenGet: true,
		CheckWhenPut: true,
	})
}

// NewWithSpec returns a new pool
func NewWithSpec(spec Spec) *Pool {
	p := &Pool{
		initSize:     spec.InitSize,
		maxSize:      spec.MaxSize,
		new:          spec.New,
		checkWhenPut: spec.CheckWhenPut,
		checkWhenGet: spec.CheckWhenGet,
		store:        make(chan PoolObject, spec.MaxSize),
		cond:         sync.NewCond(&sync.Mutex{}),
	}
	if err := p.init(); err != nil {
		logger.Errorf("new pool failed %v", err)
		return nil
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

	return nil
}

// initializes the pool with initSize of objects
func (p *Pool) init() error {
	for i := 0; i < int(p.initSize); i++ {
		iPoolObject, err := p.new()
		if err != nil || !iPoolObject.HealthCheck() {
			logger.Errorf("create pool object failed or object of pool is not healthy when init pool:, err %v", err)
			continue
		}
		p.size++
		p.store <- iPoolObject
	}
	return nil
}

// Get returns an object from the pool,
// if no free object, it will create a new one if the pool is not full
// if the pool is full, it will block until an object is returned to the pool
func (p *Pool) Get(ctx context.Context) (PoolObject, error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case poolObject := <-p.store:
			if p.checkWhenGet && !poolObject.HealthCheck() {
				p.destroyIPoolObject(poolObject)
				if poolObject = p.createIPoolObject(); poolObject != nil {
					return poolObject, nil
				}
				continue
			}
			return poolObject, nil
		default:
			if iPoolObject := p.createIPoolObject(); iPoolObject != nil {
				return iPoolObject, nil
			}
			p.cond.Wait()
		}
	}
}

func (p *Pool) createIPoolObject() PoolObject {
	for {
		size := atomic.LoadInt32(&p.size)
		if size >= p.maxSize {
			return nil
		}
		if atomic.CompareAndSwapInt32(&p.size, size, size+1) {
			break
		}
	}

	if iPoolObject, err := p.new(); err == nil {
		return iPoolObject
	}

	atomic.AddInt32(&p.size, -1)
	return nil
}

func (p *Pool) destroyIPoolObject(object PoolObject) {
	atomic.AddInt32(&p.size, -1)
	object.Destroy()
}

// Put return the object to the pool
func (p *Pool) Put(obj PoolObject) {
	if obj == nil {
		return
	}
	defer p.cond.Broadcast()
	if p.checkWhenPut && !obj.HealthCheck() {
		p.destroyIPoolObject(obj)
		return
	}
	p.store <- obj
	return
}

// Destroy destroys the pool and clean all the objects
func (p *Pool) Destroy() {
	close(p.store)
	for iPoolObject := range p.store {
		p.destroyIPoolObject(iPoolObject)
	}
	return
}
