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
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"math/rand"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// helper functions

const wasmMemory = "memory"

func (vm *WasmVM) readDataFromWasm(addr int32) []byte {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	size := int32(binary.LittleEndian.Uint32(mem[addr:]))
	data := make([]byte, size)
	copy(data, mem[addr+4:])
	return data
}

func (vm *WasmVM) writeDataToWasm(data []byte) int32 {
	vaddr, e := vm.fnAlloc.Call(vm.store, int32(len(data)+4))
	if e != nil {
		panic(e)
	}

	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	addr := vaddr.(int32)

	binary.LittleEndian.PutUint32(mem[addr:], uint32(len(data)))
	copy(mem[addr+4:], data)

	return addr
}

func (vm *WasmVM) readStringFromWasm(addr int32) string {
	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	size := int32(binary.LittleEndian.Uint32(mem[addr:]))
	data := make([]byte, size-1)
	copy(data, mem[addr+4:])
	return string(data)
}

// a string is serialized as 4 byte length + content + trailing zero
func (vm *WasmVM) writeStringToWasm(s string) int32 {
	vaddr, e := vm.fnAlloc.Call(vm.store, int32(len(s)+4+1))
	if e != nil {
		panic(e)
	}

	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	addr := vaddr.(int32)

	binary.LittleEndian.PutUint32(mem[addr:], uint32(len(s)+1))
	copy(mem[addr+4:], s)
	mem[addr+4+int32(len(s))] = 0

	return addr
}

func (vm *WasmVM) writeStringArrayToWasm(strs []string) int32 {
	size := 4 // 4 is sizeof(int32)
	for _, s := range strs {
		size += len(s) + 4 + 1
	}

	vaddr, e := vm.fnAlloc.Call(vm.store, int32(size))
	if e != nil {
		panic(e)
	}

	mem := vm.inst.GetExport(vm.store, wasmMemory).Memory().UnsafeData(vm.store)
	addr := vaddr.(int32)
	pos := int(addr)

	binary.LittleEndian.PutUint32(mem[pos:], uint32(len(strs)))
	pos += 4

	for _, s := range strs {
		binary.LittleEndian.PutUint32(mem[pos:], uint32(len(s)+1))
		pos += 4
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

func (vm *WasmVM) readClusterKeyFromWasm(addr int32) string {
	key := vm.readStringFromWasm(addr)
	return vm.host.dataPrefix + key
}

// request functions

func (vm *WasmVM) hostRequestGetRealIP() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).RealIP()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetMethod() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Method()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetMethod(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).SetMethod(v)
}

func (vm *WasmVM) hostRequestGetScheme() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Scheme()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetHost() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Host()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetHost(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).SetHost(v)
}

func (vm *WasmVM) hostRequestGetPath() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Path()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetPath(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).SetPath(v)
}

func (vm *WasmVM) hostRequestGetEscapedPath() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Std().URL.EscapedPath()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetQuery() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Std().URL.RawQuery
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestSetQuery(addr int32) {
	v := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).Std().URL.RawQuery = v
}

func (vm *WasmVM) hostRequestGetFragment() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Std().URL.Fragment
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestGetProto() int32 {
	v := vm.ctx.GetInputRequest().(*httpprot.Request).Proto()
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestAddHeader(nameAddr, valueAddr int32) {
	name := vm.readStringFromWasm(nameAddr)
	val := vm.readStringFromWasm(valueAddr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).Header().Add(name, val)
}

func (vm *WasmVM) hostRequestSetHeader(nameAddr, valueAddr int32) {
	name := vm.readStringFromWasm(nameAddr)
	value := vm.readStringFromWasm(valueAddr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).Header().Set(name, value)
}

func (vm *WasmVM) hostRequestGetHeader(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	v := vm.ctx.GetInputRequest().(*httpprot.Request).HTTPHeader().Get(name)
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostRequestDelHeader(addr int32) {
	name := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).Header().Del(name)
}

func (vm *WasmVM) hostRequestGetAllHeader() int32 {
	h := vm.ctx.GetInputRequest().(*httpprot.Request).HTTPHeader()
	return vm.writeHeaderToWasm(h)
}

func (vm *WasmVM) hostRequestSetAllHeader(addr int32) {
	h := vm.readHeaderFromWasm(addr)
	vm.ctx.GetOutputRequest().(*httpprot.Request).Std().Header = h
}

func (vm *WasmVM) hostRequestGetCookie(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	c, e := vm.ctx.GetInputRequest().(*httpprot.Request).Cookie(name)
	if e != nil && e != http.ErrNoCookie {
		panic(e)
	}
	return vm.writeStringToWasm(c.String())
}

func (vm *WasmVM) hostRequestGetAllCookie() int32 {
	var cookies []string
	for _, c := range vm.ctx.GetInputRequest().(*httpprot.Request).Cookies() {
		cookies = append(cookies, c.String())
	}
	return vm.writeStringArrayToWasm(cookies)
}

func (vm *WasmVM) hostRequestAddCookie(addr int32) {
	str := vm.readStringFromWasm(addr)
	h := http.Header{}
	h.Add("Cookie", str)
	r := http.Request{Header: h}
	for _, c := range r.Cookies() {
		vm.ctx.GetOutputRequest().(*httpprot.Request).AddCookie(c)
	}
}

func (vm *WasmVM) hostRequestGetBody() int32 {
	r := vm.ctx.GetInputRequest().(*httpprot.Request)
	return vm.writeDataToWasm(r.RawPayload())
}

func (vm *WasmVM) hostRequestSetBody(addr int32) {
	body := vm.readDataFromWasm(addr)
	req := vm.ctx.GetOutputRequest()
	if req.IsStream() {
		if c, ok := req.GetPayload().(io.Closer); ok {
			c.Close()
		}
	}
	req.SetPayload(body)
}

// response functions

func (vm *WasmVM) hostResponseGetStatusCode() int32 {
	return int32(vm.ctx.GetInputResponse().(*httpprot.Response).StatusCode())
}

func (vm *WasmVM) hostResponseSetStatusCode(code int32) {
	vm.ctx.GetOutputResponse().(*httpprot.Response).SetStatusCode(int(code))
}

func (vm *WasmVM) hostResponseAddHeader(nameAddr, valueAddr int32) {
	name := vm.readStringFromWasm(nameAddr)
	value := vm.readStringFromWasm(valueAddr)
	vm.ctx.GetOutputResponse().Header().Add(name, value)
}

func (vm *WasmVM) hostResponseSetHeader(nameAddr, valueAddr int32) {
	name := vm.readStringFromWasm(nameAddr)
	value := vm.readStringFromWasm(valueAddr)
	vm.ctx.GetOutputResponse().Header().Set(name, value)
}

func (vm *WasmVM) hostResponseGetHeader(addr int32) int32 {
	name := vm.readStringFromWasm(addr)
	v := vm.ctx.GetInputResponse().(*httpprot.Response).HTTPHeader().Get(name)
	return vm.writeStringToWasm(v)
}

func (vm *WasmVM) hostResponseDelHeader(addr int32) {
	name := vm.readStringFromWasm(addr)
	vm.ctx.GetOutputResponse().Header().Del(name)
}

func (vm *WasmVM) hostResponseGetAllHeader() int32 {
	h := vm.ctx.GetInputResponse().(*httpprot.Response).HTTPHeader()
	return vm.writeHeaderToWasm(h)
}

func (vm *WasmVM) hostResponseSetAllHeader(addr int32) {
	h := vm.readHeaderFromWasm(addr)
	vm.ctx.GetOutputResponse().(*httpprot.Response).Std().Header = h
}

func (vm *WasmVM) hostResponseSetCookie(addr int32) {
	str := vm.readStringFromWasm(addr)
	h := http.Header{}
	h.Add("Set-Cookie", str)
	r := http.Response{Header: h}
	for _, c := range r.Cookies() {
		vm.ctx.GetOutputResponse().(*httpprot.Response).SetCookie(c)
	}
}

func (vm *WasmVM) hostResponseGetBody() int32 {
	r := vm.ctx.GetInputResponse()
	return vm.writeDataToWasm(r.RawPayload())
}

func (vm *WasmVM) hostResponseSetBody(addr int32) {
	body := vm.readDataFromWasm(addr)
	resp := vm.ctx.GetOutputResponse()
	if resp.IsStream() {
		if c, ok := resp.GetPayload().(io.Closer); ok {
			c.Close()
		}
	}
	resp.SetPayload(body)
}

// cluster data functions

func (vm *WasmVM) hostClusterGetBinary(addr int32) int32 {
	var val []byte
	key := vm.readClusterKeyFromWasm(addr)
	data := vm.host.Data()
	if kv := data[key]; kv != nil {
		val, _ = base64.StdEncoding.DecodeString(string(kv.Value))
	}
	return vm.writeDataToWasm(val)
}

func (vm *WasmVM) hostClusterPutBinary(keyAddr, valAddr int32) {
	key := vm.readClusterKeyFromWasm(keyAddr)
	val := vm.readDataFromWasm(valAddr)
	v := base64.StdEncoding.EncodeToString(val)
	if e := vm.host.Cluster().Put(key, v); e != nil {
		panic(e)
	}
}

func (vm *WasmVM) hostClusterGetString(addr int32) int32 {
	val := ""
	key := vm.readClusterKeyFromWasm(addr)
	data := vm.host.Data()
	if kv := data[key]; kv != nil {
		val = string(kv.Value)
	}
	return vm.writeStringToWasm(val)
}

func (vm *WasmVM) hostClusterPutString(keyAddr, valAddr int32) {
	key := vm.readClusterKeyFromWasm(keyAddr)
	val := vm.readStringFromWasm(valAddr)
	if e := vm.host.Cluster().Put(key, val); e != nil {
		panic(e)
	}
}

func (vm *WasmVM) hostClusterGetInteger(addr int32) int64 {
	key := vm.readClusterKeyFromWasm(addr)
	data := vm.host.Data()
	kv := data[key]
	if kv == nil {
		return 0
	}
	v, e := strconv.ParseInt(string(kv.Value), 0, 64)
	if e != nil {
		return 0
	}
	return v
}

func (vm *WasmVM) hostClusterPutInteger(keyAddr int32, val int64) {
	key := vm.readClusterKeyFromWasm(keyAddr)
	v := strconv.FormatInt(val, 10)
	if e := vm.host.Cluster().Put(key, v); e != nil {
		panic(e)
	}
}

func (vm *WasmVM) hostClusterAddInteger(keyAddr int32, addend int64) int64 {
	key := vm.readClusterKeyFromWasm(keyAddr)
	c := vm.host.Cluster()
	result := int64(0)

	addFunc := func(stm concurrency.STM) error {
		s := stm.Get(key)
		if v, e := strconv.ParseInt(s, 0, 64); e != nil {
			result = 0
		} else {
			result = v
		}
		result += addend

		stm.Put(key, strconv.FormatInt(result, 10))
		return nil
	}

	if e := c.STM(addFunc); e != nil {
		panic(e)
	}

	return result
}

func (vm *WasmVM) hostClusterGetFloat(addr int32) float64 {
	key := vm.readClusterKeyFromWasm(addr)
	data := vm.host.Data()
	kv := data[key]
	if kv == nil {
		return 0
	}
	v, e := strconv.ParseFloat(string(kv.Value), 64)
	if e != nil {
		return 0
	}
	return v
}

func (vm *WasmVM) hostClusterPutFloat(keyAddr int32, val float64) {
	key := vm.readClusterKeyFromWasm(keyAddr)
	v := strconv.FormatFloat(val, 'g', -1, 64)
	if e := vm.host.Cluster().Put(key, v); e != nil {
		panic(e)
	}
}

func (vm *WasmVM) hostClusterAddFloat(keyAddr int32, addend float64) float64 {
	key := vm.readClusterKeyFromWasm(keyAddr)
	c := vm.host.Cluster()
	result := float64(0)

	addFunc := func(stm concurrency.STM) error {
		s := stm.Get(key)
		if v, e := strconv.ParseFloat(s, 64); e != nil {
			result = 0
		} else {
			result = v
		}
		result += addend

		v := strconv.FormatFloat(result, 'g', -1, 64)
		stm.Put(key, v)
		return nil
	}

	if e := c.STM(addFunc); e != nil {
		panic(e)
	}

	return result
}

func (vm *WasmVM) hostClusterCountKey(prefixAddr int32) int32 {
	prefix := vm.readClusterKeyFromWasm(prefixAddr)
	data := vm.host.Data()
	count := int32(0)

	for k := range data {
		if strings.HasPrefix(k, prefix) {
			count++
		}
	}
	return count
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

	defineFunc("host_resp_set_cookie", vm.hostResponseSetCookie)

	defineFunc("host_resp_get_body", vm.hostResponseGetBody)
	defineFunc("host_resp_set_body", vm.hostResponseSetBody)

	// cluster data functions
	defineFunc("host_cluster_get_binary", vm.hostClusterGetBinary)
	defineFunc("host_cluster_put_binary", vm.hostClusterPutBinary)

	defineFunc("host_cluster_get_string", vm.hostClusterGetString)
	defineFunc("host_cluster_put_string", vm.hostClusterPutString)

	defineFunc("host_cluster_get_integer", vm.hostClusterGetInteger)
	defineFunc("host_cluster_put_integer", vm.hostClusterPutInteger)
	defineFunc("host_cluster_add_integer", vm.hostClusterAddInteger)

	defineFunc("host_cluster_get_float", vm.hostClusterGetFloat)
	defineFunc("host_cluster_put_float", vm.hostClusterPutFloat)
	defineFunc("host_cluster_add_float", vm.hostClusterAddFloat)

	defineFunc("host_cluster_count_key", vm.hostClusterCountKey)

	// misc functions
	defineFunc("host_add_tag", vm.hostAddTag)
	defineFunc("host_log", vm.hostLog)
	defineFunc("host_get_unix_time_in_ms", vm.hostGetUnixTimeInMs)
	defineFunc("host_rand", vm.hostRand)
}
