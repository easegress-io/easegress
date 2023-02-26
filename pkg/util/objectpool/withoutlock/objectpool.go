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

// Package withoutlock provides Pool of IPoolObject base on sync-free
package withoutlock

import (
	"context"
	"fmt"
	"github.com/megaease/easegress/pkg/logger"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

const maxBackoff = 16

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
	backoff := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case poolObject := <-p.store:
			//fmt.Printf("[%d] id %d size %d got oj from channel\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
			if p.checkWhenGet && !poolObject.HealthCheck() {
				//fmt.Printf("[%d] id %d size %d would destroy oj\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
				p.destroyIPoolObject(poolObject)
				if poolObject = p.createIPoolObject(); poolObject != nil {
					//fmt.Printf("[%d] id %d size %d created new ob after destroy\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
					return poolObject, nil
				}
				continue
			}
			//fmt.Printf("[%d] id %d size %d checked ob from channel\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
			return poolObject, nil
		default:
			//fmt.Printf("[%d] id %d size %d enter default case\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
			if iPoolObject := p.createIPoolObject(); iPoolObject != nil {
				//fmt.Printf("[%d] id %d size %d created new oj when default case\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
				return iPoolObject, nil
			}
			//fmt.Printf("[%d] id %d size %d would go backoff\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
			for i := 0; i < backoff; i++ {
				runtime.Gosched()
			}
			if backoff < maxBackoff {
				backoff <<= 1
			}
		}
	}
}

func (p *Pool) createIPoolObject() IPoolObject {
	if atomic.AddInt32(&p.size, 1) > p.maxSize {
		atomic.AddInt32(&p.size, -1)
		return nil
	}

	for i := 0; i <= int(p.maxSize); i++ {
		if iPoolObject, err := p.new(); err == nil && (!p.checkWhenGet || (p.checkWhenGet && iPoolObject.HealthCheck())) {
			return iPoolObject
		}
	}
	atomic.AddInt32(&p.size, -1)
	return nil
}

func goID() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

func (p *Pool) destroyIPoolObject(object IPoolObject) {
	atomic.AddInt32(&p.size, -1)
	object.Destroy()
}

// Put return the object to the pool
func (p *Pool) Put(obj IPoolObject) {
	if obj == nil {
		return
	}
	if p.checkWhenPut && !obj.HealthCheck() {
		//fmt.Printf("[%d] id %d size %d destroy ob when put back\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
		p.destroyIPoolObject(obj)
		return
	}
	//fmt.Printf("[%d] id %d size %d done put\n", time.Now().UnixMicro(), goID(), atomic.LoadInt32(&p.size))
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
