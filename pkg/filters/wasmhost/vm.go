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
	"fmt"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
)

// WasmVM represents a wasm VM
type WasmVM struct {
	host    *WasmHost
	ctx     *context.Context
	store   *wasmtime.Store
	inst    *wasmtime.Instance
	fnRun   *wasmtime.Func
	fnAlloc *wasmtime.Func
	fnFree  *wasmtime.Func
}

// Interrupt interrupts the execution of wasm code
func (vm *WasmVM) Interrupt() {
	vm.store.Engine.IncrementEpoch()
}

// Run executes the wasm code
func (vm *WasmVM) Run() interface{} {
	r, e := vm.fnRun.Call(vm.store)
	if e != nil {
		panic(e)
	}
	return r
}

func (vm *WasmVM) exportWasmFuncs() error {
	if extern := vm.inst.GetExport(vm.store, "wasm_run"); extern == nil {
		return fmt.Errorf("wasm code hasn't export function 'wasm_run'")
	} else if fn := extern.Func(); fn == nil {
		return fmt.Errorf("'wasm_run' exported by wasm code is not a function")
	} else {
		vm.fnRun = fn
	}

	if extern := vm.inst.GetExport(vm.store, "wasm_alloc"); extern == nil {
		return fmt.Errorf("wasm code hasn't export function 'wasm_alloc'")
	} else if fn := extern.Func(); fn == nil {
		return fmt.Errorf("'wasm_alloc' exported by wasm code is not a function")
	} else {
		vm.fnAlloc = fn
	}

	if extern := vm.inst.GetExport(vm.store, "wasm_free"); extern == nil {
		return fmt.Errorf("wasm code hasn't export function 'wasm_free'")
	} else if fn := extern.Func(); fn == nil {
		return fmt.Errorf("'wasm_free' exported by wasm code is not a function")
	} else {
		vm.fnFree = fn
	}

	return nil
}

func (vm *WasmVM) callInit(params []string) (err error) {
	extern := vm.inst.GetExport(vm.store, "wasm_init")
	if extern == nil {
		// this is fine, wasm_init is optional
		return nil
	}
	fn := extern.Func()
	if fn == nil {
		return fmt.Errorf("'wasm_init' exported by wasm code is not a function")
	}

	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	addr := vm.writeStringArrayToWasm(params)
	_, err = fn.Call(vm.store, addr)
	return
}

func newWasmVM(host *WasmHost, engine *wasmtime.Engine, module *wasmtime.Module, params []string) (*WasmVM, error) {
	store := wasmtime.NewStore(engine)
	store.SetEpochDeadline(1)
	vm := &WasmVM{host: host, store: store}

	linker := wasmtime.NewLinker(engine)
	vm.importHostFuncs(linker)

	e := linker.DefineWasi()
	if e != nil {
		return nil, e
	}

	inst, e := linker.Instantiate(store, module)
	if e != nil {
		return nil, e
	}
	vm.inst = inst

	if e = vm.exportWasmFuncs(); e != nil {
		return nil, e
	}

	e = vm.callInit(params)
	if e != nil {
		return nil, e
	}

	return vm, nil
}

// WasmVMPool is a pool of wasm VMs
type WasmVMPool struct {
	host   *WasmHost
	chVM   chan *WasmVM
	engine *wasmtime.Engine
	module *wasmtime.Module
	params []string
}

// NewWasmVMPool creates a wasm VM pool according the spec of 'host' which execute 'code'
func NewWasmVMPool(host *WasmHost, code []byte) (*WasmVMPool, error) {
	cfg := wasmtime.NewConfig()
	cfg.SetEpochInterruption(true)
	engine := wasmtime.NewEngineWithConfig(cfg)
	module, e := wasmtime.NewModule(engine, code)
	if e != nil {
		logger.Errorf("failed to create wasm module: %v", e)
		return nil, e
	}

	p := &WasmVMPool{host: host, engine: engine, module: module}
	for k, v := range host.spec.Parameters {
		p.params = append(p.params, k, v)
	}

	p.chVM = make(chan *WasmVM, host.spec.MaxConcurrency)
	for i := int32(0); i < host.spec.MaxConcurrency; i++ {
		vm, e := newWasmVM(p.host, p.engine, p.module, p.params)
		if e != nil {
			logger.Errorf("failed to create wasm VM: %v", e)
		}
		p.chVM <- vm
	}

	return p, nil
}

// Get gets a wasm VM from the pool
func (p *WasmVMPool) Get() *WasmVM {
	vm := <-p.chVM
	if vm != nil {
		return vm
	}

	// vm is nil, we need create a new one
	vm, e := newWasmVM(p.host, p.engine, p.module, p.params)
	if e != nil {
		p.chVM <- nil
		logger.Errorf("failed to create wasm VM: %v", e)
		return nil
	}

	return vm
}

// Put puts a wasm VM to the pool, putting a nil VM is allowed
// and will cause p.Get to create a new VM later
func (p *WasmVMPool) Put(vm *WasmVM) {
	p.chVM <- vm
}
