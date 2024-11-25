//go:build wasmhost
// +build wasmhost

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

package wasmhost

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

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

const (
	// Kind is the kind of WasmHost.
	Kind          = "WasmHost"
	maxWasmResult = 9
)

var (
	resultOutOfVM   = "outOfVM"
	resultWasmError = "wasmError"
	results         = []string{resultOutOfVM, resultWasmError}
)

func wasmResultToFilterResult(r int32) string {
	if r == 0 {
		return ""
	}
	return fmt.Sprintf("wasmResult%d", r)
}

var kind = &filters.Kind{
	Name:        Kind,
	Description: "WasmHost implements a host environment for WebAssembly",
	Results:     results,
	DefaultSpec: func() filters.Spec {
		return &Spec{
			MaxConcurrency: 10,
			Timeout:        "100ms",
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &WasmHost{spec: spec.(*Spec)}
	},
}

func init() {
	for i := int32(1); i <= maxWasmResult; i++ {
		results = append(results, wasmResultToFilterResult(i))
	}

	kind.Results = results
	filters.Register(kind)
}

type (
	// Spec is the spec for WasmHost
	Spec struct {
		filters.BaseSpec `json:",inline"`

		MaxConcurrency int32             `json:"maxConcurrency" jsonschema:"required,minimum=1"`
		Code           string            `json:"code" jsonschema:"required"`
		Timeout        string            `json:"timeout" jsonschema:"required,format=duration"`
		Parameters     map[string]string `json:"parameters,omitempty"`
		timeout        time.Duration
	}

	// WasmHost is the WebAssembly filter
	WasmHost struct {
		spec *Spec

		code       []byte
		dataPrefix string
		data       atomic.Value
		vmPool     atomic.Value
		chStop     chan struct{}

		numOfRequest   int64
		numOfWasmError int64
	}

	// Status is the status of WasmHost
	Status struct {
		Health         string `json:"health"`
		NumOfRequest   int64  `json:"numOfRequest"`
		NumOfWasmError int64  `json:"numOfWasmError"`
	}
)

// Name returns the name of the WasmHost filter instance.
func (wh *WasmHost) Name() string {
	return wh.spec.Name()
}

// Kind returns the kind of WasmHost.
func (wh *WasmHost) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the WasmHost
func (wh *WasmHost) Spec() filters.Spec {
	return wh.spec
}

// Cluster returns the cluster
func (wh *WasmHost) Cluster() cluster.Cluster {
	return wh.spec.Super().Cluster()
}

// Data returns the shared data
func (wh *WasmHost) Data() map[string]*mvccpb.KeyValue {
	d := wh.data.Load()
	if d == nil {
		return map[string]*mvccpb.KeyValue{}
	}
	return d.(map[string]*mvccpb.KeyValue)
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
			if p == strings.ToLower(str[:len(p)]) {
				return true
			}
		}
	}
	return false
}

func (wh *WasmHost) readWasmCode() ([]byte, error) {
	if isURL(wh.spec.Code) {
		return readWasmCodeFromURL(wh.spec.Code)
	}
	if _, e := os.Stat(wh.spec.Code); e == nil {
		return os.ReadFile(wh.spec.Code)
	}
	return base64.StdEncoding.DecodeString(wh.spec.Code)
}

func (wh *WasmHost) loadWasmCode() error {
	code, e := wh.readWasmCode()
	if e != nil {
		logger.Errorf("failed to load wasm code: %v", e)
		return e
	}

	if len(wh.code) > 0 && bytes.Equal(wh.code, code) {
		return nil
	}

	p, e := NewWasmVMPool(wh, code)
	if e != nil {
		logger.Errorf("failed to create wasm VM pool: %v", e)
		return e
	}
	wh.code = code

	wh.vmPool.Store(p)
	return nil
}

func (wh *WasmHost) watchWasmCode() {
	var (
		chWasm <-chan *string
		syncer cluster.Syncer
		err    error
	)

	for {
		c := wh.Cluster()
		syncer, err = c.Syncer(time.Minute)
		if err == nil {
			chWasm, err = syncer.Sync(c.Layout().WasmCodeEvent())
			if err == nil {
				break
			}
		}
		logger.Errorf("failed to watch wasm code event: %v", err)
		select {
		case <-time.After(10 * time.Second):
		case <-wh.chStop:
			return
		}
	}

	for {
		select {
		case <-chWasm:
			err = wh.loadWasmCode()

		case <-time.After(30 * time.Second):
			if err != nil || len(wh.code) == 0 {
				err = wh.loadWasmCode()
			}

		case <-wh.chStop:
			return
		}
	}
}

func (wh *WasmHost) watchWasmData() {
	var (
		chWasm <-chan map[string]*mvccpb.KeyValue
		syncer cluster.Syncer
		err    error
	)

	for {
		c := wh.Cluster()
		syncer, err = c.Syncer(time.Minute)
		if err == nil {
			chWasm, err = syncer.SyncRawPrefix(wh.dataPrefix)
			if err == nil {
				break
			}
		}
		logger.Errorf("failed to watch wasm data: %v", err)
		select {
		case <-time.After(10 * time.Second):
		case <-wh.chStop:
			return
		}
	}

	for {
		select {
		case data := <-chWasm:
			wh.data.Store(data)

		case <-wh.chStop:
			return
		}
	}
}

func (wh *WasmHost) reload() {
	spec := wh.spec

	wh.dataPrefix = wh.Cluster().Layout().WasmDataPrefix(spec.Pipeline(), spec.Name())

	wh.spec.timeout, _ = time.ParseDuration(wh.spec.Timeout)
	wh.chStop = make(chan struct{})

	wh.loadWasmCode()
	go wh.watchWasmCode()
	go wh.watchWasmData()
}

// Init initializes WasmHost.
func (wh *WasmHost) Init() {
	wh.reload()
}

// Inherit inherits previous generation of WasmHost.
func (wh *WasmHost) Inherit(previousGeneration filters.Filter) {
	wh.reload()
}

// Handle handles HTTP request
func (wh *WasmHost) Handle(ctx *context.Context) (result string) {
	// we must save the pool to a local variable for later use as it will be
	// replaced when updating the wasm code
	var pool *WasmVMPool
	if p := wh.vmPool.Load(); p == nil {
		ctx.AddTag("wasm VM pool is not initialized")
		return resultOutOfVM
	} else {
		pool = p.(*WasmVMPool)
	}

	// get a free wasm VM and attach the ctx to it
	vm := pool.Get()
	if vm == nil {
		ctx.AddTag("failed to get a wasm VM")
		return resultOutOfVM
	}
	vm.ctx = ctx
	atomic.AddInt64(&wh.numOfRequest, 1)

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
			atomic.AddInt64(&wh.numOfWasmError, 1)
			vm = nil
		}

		pool.Put(vm)
	}()

	// start another goroutine to interrupt the wasm execution
	wg.Add(1)
	go func() {
		defer wg.Done()

		timer := time.NewTimer(wh.spec.timeout)

		select {
		case <-chCancelInterrupt:
			break
		case <-timer.C:
			vm.Interrupt()
			vm = nil
			break
		case <-ctx.GetInputRequest().(*httpprot.Request).Context().Done():
			vm.Interrupt()
			vm = nil
			break
		}

		if !timer.Stop() {
			<-timer.C
		}
	}()

	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	ctx.SetOutputResponse(resp)

	r := vm.Run() // execute wasm code
	n, ok := r.(int32)
	if !ok || n < 0 || n > maxWasmResult {
		panic(fmt.Errorf("invalid wasm result: %v", r))
	}

	return wasmResultToFilterResult(n)
}

// Status returns Status generated by the filter.
func (wh *WasmHost) Status() interface{} {
	p := wh.vmPool.Load()
	s := &Status{}
	if p == nil {
		s.Health = "VM pool is not initialized"
	} else {
		s.Health = "ready"
	}

	s.NumOfRequest = atomic.LoadInt64(&wh.numOfRequest)
	s.NumOfWasmError = atomic.LoadInt64(&wh.numOfWasmError)
	return s
}

// Close closes WasmHost.
func (wh *WasmHost) Close() {
	close(wh.chStop)
}
