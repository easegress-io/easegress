// +build wasmhost

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

package wasmhost

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/megaease/easegress/pkg/logger"
)

// helper functions

const wasmMemory = "memory"

func (vm *WasmVM) readDataFromWasm(addr int32) []byte {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	reader := bytes.NewReader(mem[addr:])
	var size int32
	if e := binary.Read(reader, binary.LittleEndian, &size); e != nil {
		panic(e)
	}

	data := make([]byte, size)
	if _, e := reader.Read(data); e != nil {
		panic(e)
	}

	return data
}

func (vm *WasmVM) writeDataToWasm(data []byte) int32 {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	vaddr, e := vm.fnAlloc.Call(vm.store, len(data)+4)
	if e != nil {
		panic(e)
	}
	addr := vaddr.(int32)
	pos := int(addr)

	binary.LittleEndian.PutUint32(mem[pos:], uint32(len(data)))
	pos += 4
	copy(mem[pos:], data)

	return addr
}

func (vm *WasmVM) readStringFromWasm(addr int32) string {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)

	start := addr
	for mem[addr] != 0 {
		addr++
	}
	data := make([]byte, addr-start)
	copy(data, mem[start:addr])

	return string(data)
}

func (vm *WasmVM) writeStringToWasm(s string) int32 {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	vaddr, e := vm.fnAlloc.Call(vm.store, len(s)+1)
	if e != nil {
		panic(e)
	}

	addr := vaddr.(int32)
	copy(mem[addr:], []byte(s))
	mem[addr+int32(len(s))] = 0

	return addr
}

func (vm *WasmVM) readMultipleStringFromWasm(addr, count int32) []string {
	result := make([]string, 0, count)
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)

	for i := int32(0); i < count; i++ {
		start := addr
		for mem[addr] != 0 {
			addr++
		}
		data := make([]byte, addr-start)
		copy(data, mem[start:addr])
		result = append(result, string(data))
		addr++
	}

	return result
}

func (vm *WasmVM) writeMultipleStringToWasm(strs []string) int32 {
	size := 4 // 4 is sizeof(int32)
	for _, s := range strs {
		size += len(s) + 1
	}

	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	vaddr, e := vm.fnAlloc.Call(vm.store, int32(size))
	if e != nil {
		panic(e)
	}
	addr := vaddr.(int32)
	pos := int(addr)

	binary.LittleEndian.PutUint32(mem[pos:], uint32(len(strs)))
	pos += 4

	for _, s := range strs {
		copy(mem[pos:], []byte(s))
		pos += len(s)
		mem[pos] = 0
		pos++
	}

	return addr
}

func (vm *WasmVM) writeHeaderToWasm(h http.Header) int32 {
	var buf bytes.Buffer
	h.Write(&buf)
	return vm.writeStringToWasm(buf.String())
}

func (vm *WasmVM) readHeaderFromWasm(addr int32) http.Header {
	str := vm.readStringFromWasm(addr) + "\r\n"
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(str)))
	h, e := r.ReadMIMEHeader()
	if e != nil {
		panic(e)
	}
	return http.Header(h)
}

// request functions

func (vm *WasmVM) hostRequestGetRealIP() int32 {
	v := vm.ctx.Request().RealIP()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetMethod() int32 {
	v := vm.ctx.Request().Method()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetMethod(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.Request().SetMethod(v)
}

func (vm *WasmVM) hostRequestGetScheme() int32 {
	v := vm.ctx.Request().Scheme()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetHost() int32 {
	v := vm.ctx.Request().Host()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetHost(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.Request().SetHost(v)
}

func (vm *WasmVM) hostRequestGetPath() int32 {
	v := vm.ctx.Request().Path()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetPath(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.Request().SetPath(v)
}

func (vm *WasmVM) hostRequestGetEscapedPath() int32 {
	v := vm.ctx.Request().EscapedPath()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetQuery() int32 {
	v := vm.ctx.Request().Query()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetQuery(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.Request().SetQuery(v)
}

func (vm *WasmVM) hostRequestGetFragment() int32 {
	v := vm.ctx.Request().Fragment()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetProto() int32 {
	v := vm.ctx.Request().Proto()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestAddHeader(addr int32) {
	strs := vm.readMultipleStringFromWasm(addr, 2)
	vm.ctx.Request().Header().Add(strs[0], strs[1])
}

func (vm *WasmVM) hostRequestSetHeader(addr int32) {
	strs := vm.readMultipleStringFromWasm(addr, 2)
	vm.ctx.Request().Header().Set(strs[0], strs[1])
}

func (vm *WasmVM) hostRequestGetHeader(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	v := vm.ctx.Request().Header().Get(name)
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestDelHeader(addr int32) {
	name := vm.readStringFromWasm(addr)
	vm.ctx.Request().Header().Del(name)
}

func (vm *WasmVM) hostRequestGetAllHeader() int32 {
	h := vm.ctx.Request().Header().Std()
	return vm.writeHeaderToWasm(h)
}

func (vm *WasmVM) hostRequestSetAllHeader(addr int32) {
	h := vm.readHeaderFromWasm(addr)
	vm.ctx.Request().Header().SetFromStd(h)
}

func (vm *WasmVM) hostRequestGetCookie(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	c, e := vm.ctx.Request().Cookie(name)
	if e != nil && e != http.ErrNoCookie {
		panic(e)
	}
	return vm.writeStringToWasm(c.String())
}

func (vm *WasmVM) hostRequestGetAllCookie() int32 {
	var cookies []string
	for _, c := range vm.ctx.Request().Cookies() {
		cookies = append(cookies, c.String())
	}
	return vm.writeMultipleStringToWasm(cookies)
}

func (vm *WasmVM) hostRequestAddCookie(addr int32) {
	str := vm.readStringFromWasm(addr)
	h := http.Header{}
	h.Add("Cookie", str)
	r := http.Request{Header: h}
	for _, c := range r.Cookies() {
		vm.ctx.Request().AddCookie(c)
	}
}

func (vm *WasmVM) hostRequestGetBody() int32 {
	r := vm.ctx.Request()
	body, e := io.ReadAll(r.Body())
	if e != nil {
		panic(e)
	}
	r.SetBody(bytes.NewReader(body))

	return vm.writeDataToWasm(body)
}

func (vm *WasmVM) hostRequestSetBody(addr int32) {
	body := vm.readDataFromWasm(addr)
	vm.ctx.Request().SetBody(bytes.NewReader(body))
}

// response functions

func (vm *WasmVM) hostResponseGetStatusCode() int32 {
	return int32(vm.ctx.Response().StatusCode())
}

func (vm *WasmVM) hostResponseSetStatusCode(code int32) {
	vm.ctx.Response().SetStatusCode(int(code))
}

func (vm *WasmVM) hostResponseAddHeader(addr int32) {
	strs := vm.readMultipleStringFromWasm(addr, 2)
	vm.ctx.Response().Header().Add(strs[0], strs[1])
}

func (vm *WasmVM) hostResponseSetHeader(addr int32) {
	strs := vm.readMultipleStringFromWasm(addr, 2)
	vm.ctx.Response().Header().Set(strs[0], strs[1])
}

func (vm *WasmVM) hostResponseGetHeader(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	v := vm.ctx.Response().Header().Get(name)
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostResponseDelHeader(addr int32) {
	name := vm.readStringFromWasm(addr)
	vm.ctx.Response().Header().Del(name)
}

func (vm *WasmVM) hostResponseGetAllHeader() int32 {
	h := vm.ctx.Response().Header().Std()
	return vm.writeHeaderToWasm(h)
}

func (vm *WasmVM) hostResponseSetAllHeader(addr int32) {
	h := vm.readHeaderFromWasm(addr)
	vm.ctx.Response().Header().SetFromStd(h)
}

func (vm *WasmVM) hostResponseSetCookie(addr int32) {
	str := vm.readStringFromWasm(addr)
	h := http.Header{}
	h.Add("Set-Cookie", str)
	r := http.Response{Header: h}
	for _, c := range r.Cookies() {
		vm.ctx.Response().SetCookie(c)
	}
}

func (vm *WasmVM) hostResponseGetBody() int32 {
	r := vm.ctx.Response()
	body, e := io.ReadAll(r.Body())
	if e != nil {
		panic(e)
	}
	r.SetBody(bytes.NewReader(body))

	return vm.writeDataToWasm(body)
}

func (vm *WasmVM) hostResponseSetBody(addr int32) {
	body := vm.readDataFromWasm(addr)
	vm.ctx.Response().SetBody(bytes.NewReader(body))
}

// misc functions

func (vm *WasmVM) hostAddTag(addr int32) {
	tag := vm.readStringFromWasm(addr)
	vm.ctx.AddTag(tag)
}

func (vm *WasmVM) hostLog(level int32, addr int32) {
	msg := vm.readStringFromWasm(addr)
	switch level {
	case 0:
		logger.Debugf(msg)
	case 1:
		logger.Infof(msg)
	case 2:
		logger.Warnf(msg)
	case 3:
		logger.Errorf(msg)
	}
}

func (vm *WasmVM) hostGetUnixTimeInMs() int64 {
	return time.Now().UnixNano() / 1e6
}

func (vm *WasmVM) hostRand() float64 {
	return rand.Float64()
}

// importHostFuncs imports host functions into wasm so that user-developed wasm
// code can call these functions to interoperate with host.
func (vm *WasmVM) importHostFuncs(linker *wasmtime.Linker) {
	defineFunc := func(name string, fn interface{}) {
		if e := linker.DefineFunc(vm.store, "easegress", name, fn); e != nil {
			panic(e) // should never happen
		}
	}

	// request functions
	defineFunc("host_req_get_real_ip", vm.hostRequestGetRealIP)
	defineFunc("host_req_get_scheme", vm.hostRequestGetScheme)
	defineFunc("host_req_get_proto", vm.hostRequestGetProto)

	defineFunc("host_req_get_method", vm.hostRequestGetMethod)
	defineFunc("host_req_set_method", vm.hostRequestSetMethod)

	defineFunc("host_req_get_host", vm.hostRequestGetHost)
	defineFunc("host_req_set_host", vm.hostRequestSetHost)

	defineFunc("host_req_get_path", vm.hostRequestGetPath)
	defineFunc("host_req_set_path", vm.hostRequestSetPath)
	defineFunc("host_req_get_escaped_path", vm.hostRequestGetEscapedPath)
	defineFunc("host_req_get_query", vm.hostRequestGetQuery)
	defineFunc("host_req_set_query", vm.hostRequestSetQuery)
	defineFunc("host_req_get_fragment", vm.hostRequestGetFragment)

	defineFunc("host_req_get_header", vm.hostRequestGetHeader)
	defineFunc("host_req_get_all_header", vm.hostRequestGetAllHeader)
	defineFunc("host_req_set_header", vm.hostRequestSetHeader)
	defineFunc("host_req_set_all_header", vm.hostRequestSetAllHeader)
	defineFunc("host_req_add_header", vm.hostRequestAddHeader)
	defineFunc("host_req_del_header", vm.hostRequestDelHeader)

	defineFunc("host_req_get_cookie", vm.hostRequestGetCookie)
	defineFunc("host_req_get_all_cookie", vm.hostRequestGetAllCookie)
	defineFunc("host_req_add_cookie", vm.hostRequestAddCookie)

	defineFunc("host_req_get_body", vm.hostRequestGetBody)
	defineFunc("host_req_set_body", vm.hostRequestSetBody)

	// response functions
	defineFunc("host_resp_get_status_code", vm.hostResponseGetStatusCode)
	defineFunc("host_resp_set_status_code", vm.hostResponseSetStatusCode)

	defineFunc("host_resp_get_header", vm.hostResponseGetHeader)
	defineFunc("host_resp_get_all_header", vm.hostResponseGetAllHeader)
	defineFunc("host_resp_set_header", vm.hostResponseSetHeader)
	defineFunc("host_resp_set_all_header", vm.hostResponseSetAllHeader)
	defineFunc("host_resp_add_header", vm.hostResponseAddHeader)
	defineFunc("host_resp_del_header", vm.hostResponseDelHeader)

	defineFunc("host_req_set_cookie", vm.hostResponseSetCookie)

	defineFunc("host_resp_get_body", vm.hostResponseGetBody)
	defineFunc("host_resp_set_body", vm.hostResponseSetBody)

	// misc functions
	defineFunc("host_add_tag", vm.hostAddTag)
	defineFunc("host_log", vm.hostLog)
	defineFunc("host_get_unix_time_in_ms", vm.hostGetUnixTimeInMs)
	defineFunc("host_rand", vm.hostRand)
}
