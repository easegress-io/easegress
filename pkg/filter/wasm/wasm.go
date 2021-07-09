// +build wasmfilter

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

package wasm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of WasmFilter.
	Kind        = "WasmFilter"
	maxWasmCode = 9
)

var (
	resultOutOfVM   = "outOfVM"
	resultWasmError = "wasmError"
	results         = []string{resultOutOfVM, resultWasmError}
)

func wasmCodeToResult(code int32) string {
	if code == 0 {
		return ""
	}
	return fmt.Sprintf("wasmResult%d", code)
}

func init() {
	for i := int32(1); i <= maxWasmCode; i++ {
		results = append(results, wasmCodeToResult(i))
	}
	httppipeline.Register(&WasmFilter{})
}

type (
	Spec struct {
		MaxConcurrency int32  `yaml:"maxConcurrency" jsonschema:"required,minimum=1"`
		Code           string `yaml:"code" jsonschema:"required"`
		Timeout        string `yaml:"timeout" jsonschema:"required,format=duration"`
		timeout        time.Duration
	}

	WasmFilter struct {
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		code   []byte
		vmPool atomic.Value
		chStop chan struct{}
	}
)

// Kind returns the kind of WasmFilter.
func (f *WasmFilter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of WasmFilter.
func (f *WasmFilter) DefaultSpec() interface{} {
	return &Spec{
		MaxConcurrency: 10,
		Timeout:        "100ms",
	}
}

// Description returns the description of WasmFilter
func (f *WasmFilter) Description() string {
	return "WasmFilter implements a filter which runs web assembly"
}

// Results returns the results of WasmFilter.
func (f *WasmFilter) Results() []string {
	return results
}

func readWasmCodeFromFile(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return io.ReadAll(f)
}

func readWasmCodeFromURL(url string) ([]byte, error) {
	resp, e := http.DefaultClient.Get(url)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func isURL(str string) bool {
	// we only check the first a few bytes as str could be
	// the LONG base64 encoded wasm code
	for _, p := range []string{"http://", "https://"} {
		if len(str) > len(p) {
			s := strings.ToLower(str[:len(p)])
			if strings.HasPrefix(s, p) {
				return true
			}
		}
	}
	return false
}

func (f *WasmFilter) readWasmCode() ([]byte, error) {
	if isURL(f.spec.Code) {
		return readWasmCodeFromURL(f.spec.Code)
	}
	if _, e := os.Stat(f.spec.Code); e == nil {
		return readWasmCodeFromFile(f.spec.Code)
	}
	return base64.StdEncoding.DecodeString(f.spec.Code)
}

func (f *WasmFilter) loadWasmCode() error {
	code, e := f.readWasmCode()
	if e != nil {
		logger.Errorf("failed to load Wasm code: %v", e)
		return e
	}

	if len(f.code) > 0 && bytes.Equal(f.code, code) {
		return nil
	}

	p, e := NewWasmVMPool(f.spec.MaxConcurrency, code)
	if e != nil {
		logger.Errorf("failed to create Wasm VM pool: %v", e)
		return e
	}
	f.code = code

	f.vmPool.Store(p)
	return nil
}

func (f *WasmFilter) watchWasmCode() {
	var (
		chWasm <-chan *string
		syncer *cluster.Syncer
		err    error
	)

	for {
		c := f.pipeSpec.Super().Cluster()
		syncer, err = c.Syncer(time.Minute)
		if err == nil {
			chWasm, err = syncer.Sync(c.Layout().WasmCodeEvent())
			if err == nil {
				break
			}
		}
		logger.Errorf("failed to watch Wasm code event: %v", err)
		select {
		case <-time.After(10 * time.Second):
		case <-f.chStop:
			return
		}
	}

	for {
		select {
		case <-chWasm:
			err = f.loadWasmCode()

		case <-time.After(30 * time.Second):
			if err != nil || len(f.code) == 0 {
				err = f.loadWasmCode()
			}

		case <-f.chStop:
			return
		}
	}
}

func (f *WasmFilter) reload(pipeSpec *httppipeline.FilterSpec) {
	f.pipeSpec = pipeSpec
	f.spec = pipeSpec.FilterSpec().(*Spec)

	f.spec.timeout, _ = time.ParseDuration(f.spec.Timeout)
	f.chStop = make(chan struct{})

	f.loadWasmCode()
	go f.watchWasmCode()
}

// Init initializes WasmFilter.
func (f *WasmFilter) Init(pipeSpec *httppipeline.FilterSpec) {
	f.reload(pipeSpec)
}

// Inherit inherits previous generation of WasmFilter.
func (f *WasmFilter) Inherit(pipeSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	f.reload(pipeSpec)
}

// Handle handles HTTP request
func (f *WasmFilter) Handle(ctx context.HTTPContext) string {
	result := f.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (f *WasmFilter) handle(ctx context.HTTPContext) (result string) {
	// we must save the pool to a local variable for later use as it will be
	// replaced when updating the Wasm code
	var pool *WasmVMPool
	if p := f.vmPool.Load(); p == nil {
		ctx.AddTag("Wasm VM pool is not initialized")
		return resultOutOfVM
	} else {
		pool = p.(*WasmVMPool)
	}

	// get a free WASM VM and attach the ctx to it
	vm := pool.Get()
	if vm == nil {
		ctx.AddTag("failed to get a Wasm VM")
		return resultOutOfVM
	}
	vm.ctx = ctx

	var wg sync.WaitGroup
	chCancelInterrupt := make(chan struct{})
	defer func() {
		close(chCancelInterrupt)
		wg.Wait()

		// the VM is not usable if there's a panic, set it to nil and a new
		// VM will be created in pool.Get later
		if e := recover(); e != nil {
			logger.Errorf("recovered from wasm error: %v", e)
			result = resultWasmError
			vm = nil
		}

		pool.Put(vm)
	}()

	// start another goroutine to interrupt the wasm execution
	wg.Add(1)
	go func() {
		defer wg.Done()

		timer := time.NewTimer(f.spec.timeout)

		select {
		case <-chCancelInterrupt:
			break
		case <-timer.C:
			vm.Interrupt()
			vm = nil
			break
		case <-ctx.Done():
			vm.Interrupt()
			vm = nil
			break
		}

		if !timer.Stop() {
			<-timer.C
		}
	}()

	r := vm.Run() // execute wasm code
	code, ok := r.(int32)
	if !ok || code > maxWasmCode {
		panic(fmt.Errorf("invalid Wasm result: %v", r))
	}

	return wasmCodeToResult(code)
}

// Status returns Status genreated by Runtime.
func (f *WasmFilter) Status() interface{} {
	return nil
}

// Close closes WasmFilter.
func (f *WasmFilter) Close() {
	close(f.chStop)
}
