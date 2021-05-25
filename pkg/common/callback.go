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

package common

const (
	CallbacksInitCapacity = 20

	NORMAL_PRIORITY_CALLBACK   = "__NoRmAl_PrIoRiTy_CaLlBaCk"
	CRITICAL_PRIORITY_CALLBACK = "__CrItIcAl_PrIoRiTy_CaLlBaCk"
)

////

type NamedCallback struct {
	name     string
	callback interface{}
}

func NewNamedCallback(name string, callback interface{}) *NamedCallback {
	return &NamedCallback{
		name:     name,
		callback: callback,
	}
}

func (cb *NamedCallback) Name() string {
	return cb.name
}

func (cb *NamedCallback) Callback() interface{} {
	return cb.callback
}

func (cb *NamedCallback) SetCallback(callback interface{}) interface{} {
	oriCallback := cb.callback
	cb.callback = callback
	return oriCallback
}

type namedCallbackWithIdx struct {
	cb         *NamedCallback
	idxOfOrder int
}

type NamedCallbackSet struct {
	// critical callback takes low index, normal callback takes high index
	callbacks []*NamedCallback                 // index for access by order
	names     map[string]*namedCallbackWithIdx // index for access by callback constant

}

func NewNamedCallbackSet() *NamedCallbackSet {
	return &NamedCallbackSet{
		names:     make(map[string]*namedCallbackWithIdx, CallbacksInitCapacity),
		callbacks: make([]*NamedCallback, 0, CallbacksInitCapacity),
	}
}

func (cbs *NamedCallbackSet) CopyCallbacks() []*NamedCallback {
	ret := make([]*NamedCallback, len(cbs.callbacks))
	copy(ret, cbs.callbacks)
	return ret
}

func (cbs *NamedCallbackSet) GetCallbacks() []*NamedCallback {
	return cbs.callbacks
}

////

func AddCallback(cbs *NamedCallbackSet, name string, callback interface{}, priority string) *NamedCallbackSet {
	if cbs == nil {
		return nil
	}

	_, exists := cbs.names[name]
	if exists {
		return cbs
	}

	cb := NewNamedCallback(name, callback)
	idx := 0

	if priority == NORMAL_PRIORITY_CALLBACK {
		idx = len(cbs.callbacks)
		cbs.callbacks = append(cbs.callbacks, cb)
	} else if priority == CRITICAL_PRIORITY_CALLBACK {
		// idx = 0
		cbs.callbacks = append([]*NamedCallback{cb}, cbs.callbacks...)
	} else {
		idx = len(cbs.callbacks)
		for i, namedCallback := range cbs.callbacks {
			if namedCallback.Name() == priority {
				idx = i
				break
			}
		}

		// insert before the pos
		cbs.callbacks = append(cbs.callbacks[:idx], append([]*NamedCallback{cb}, cbs.callbacks[idx:]...)...)
	}

	cbs.names[name] = &namedCallbackWithIdx{
		cb:         cb,
		idxOfOrder: idx,
	}

	return cbs
}

func DeleteCallback(cbs *NamedCallbackSet, name string) *NamedCallbackSet {
	if cbs == nil {
		return nil
	}

	cbi, exists := cbs.names[name]
	if !exists {
		return cbs
	}

	delete(cbs.names, name)
	cbs.callbacks = append(cbs.callbacks[:cbi.idxOfOrder], cbs.callbacks[cbi.idxOfOrder+1:]...)

	for _, cbi1 := range cbs.names {
		if cbi1.idxOfOrder > cbi.idxOfOrder {
			cbi1.idxOfOrder--
		}
	}

	return cbs
}
