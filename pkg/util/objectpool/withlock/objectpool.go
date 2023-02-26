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

package withlock

import (
	"context"
	"fmt"
	"github.com/megaease/easegress/pkg/logger"
	"sync"
	"sync/atomic"
)

// IPoolObject is definition of object that managed by pool
type IPoolObject interface {
	Destroy()          // destroy the object
	HealthCheck() bool // check the object is health or not
}

type (
	// Pool manage the IPoolObject
	Pool struct {
		initSize     int32                       // initial size
		maxSize      int32                       // max size
		size         int32                       // current size
		new          func() (IPoolObject, error) // create a new object
		store        chan IPoolObject            // store the object
		lock         sync.Mutex                  // lock for sync
		checkWhenGet bool                        // whether to health check when get IPoolObject
		checkWhenPut bool                        // whether to health check when put IPoolObject
	}

	// Spec Pool's spec
	Spec struct {
		initSize     int32                       // initial size
		maxSize      int32                       // max size
		new          func() (IPoolObject, error) // create a new object
		checkWhenGet bool                        // whether to health check when get IPoolObject
		checkWhenPut bool                        // whether to health check when put IPoolObject
	}
)

// NewSimplePool returns a new pool
func NewSimplePool(initSize, maxSize int32, new func() (IPoolObject, error)) *Pool {
	return NewPoolWithSpec(Spec{
		initSize:     initSize,
		maxSize:      maxSize,
		new:          new,
		checkWhenGet: true,
		checkWhenPut: true,
	})
}

// NewPoolWithSpec returns a new pool
func NewPoolWithSpec(spec Spec) *Pool {
	p := &Pool{
		initSize:     spec.initSize,
		maxSize:      spec.maxSize,
		new:          spec.new,
		checkWhenPut: spec.checkWhenPut,
		checkWhenGet: spec.checkWhenGet,
		store:        make(chan IPoolObject, spec.maxSize),
	}
	if err := p.init(); err != nil {
		logger.Errorf("new pool failed %v", err)
		return nil
	}
	return p
}

// Validate validate
func (s *Spec) Validate() error {
	if s.initSize > s.maxSize {
		s.maxSize = s.initSize
	}
	if s.maxSize <= 0 {
		return fmt.Errorf("pool max size must be positive")
	}
	if s.initSize < 0 {
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
		atomic.AddInt32(&p.size, 1)
		p.store <- iPoolObject
	}
	return nil
}

// Get returns an object from the pool,
// if no free object, it will create a new one if the pool is not full
// if the pool is full, it will block until an object is returned to the pool
func (p *Pool) Get(ctx context.Context) (IPoolObject, error) {
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
		}
	}
}

func (p *Pool) createIPoolObject() IPoolObject {
	p.lock.Lock()
	defer p.lock.Unlock()
	for i := 0; i < int(p.maxSize-p.size); i++ {
		if iPoolObject, err := p.new(); err == nil && (!p.checkWhenGet || (p.checkWhenGet && iPoolObject.HealthCheck())) {
			p.size += 1
			return iPoolObject
		}
	}
	return nil
}

func (p *Pool) destroyIPoolObject(object IPoolObject) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.size -= 1
	object.Destroy()
}

// Put return the object to the pool
func (p *Pool) Put(obj IPoolObject) {
	if obj == nil {
		return
	}
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
