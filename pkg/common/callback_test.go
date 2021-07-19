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

import (
	"fmt"
	"testing"
)

func createFn(name string) func() string {
	return func() string {
		fmt.Println("I am a callback - " + name)
		return name
	}
}
func TestCallBack(t *testing.T) {
	foo := createFn("foo")
	bar := createFn("bar")
	name := "test"
	cb := NewNamedCallback(name, foo)
	if cb.Name() != name {
		t.Errorf("expected %s, result %s", name, cb.Name())
	}

	type FuncType func() string
	cbfn := cb.Callback()
	if f, ok := cbfn.(func() string); ok {
		if foo() != FuncType(f)() {
			t.Errorf("error")
		}
	}

	cb.SetCallback(bar)
	cbfn = cb.Callback()
	if f, ok := cbfn.(func() string); ok {
		if bar() != FuncType(f)() {
			t.Errorf("error")
		}
	}
}

func TestCallBackSet(t *testing.T) {
	set := NewNamedCallbackSet()

	x := createFn("XXX")
	y := createFn("YYY")
	z := createFn("ZZZ")
	a := createFn("AAA")
	b := createFn("BBB")

	AddCallback(set, "XXX", x, NormalPriorityCallback)
	AddCallback(set, "YYY", y, NormalPriorityCallback)
	AddCallback(set, "ZZZ", z, CriticalPriorityCallback)
	AddCallback(set, "AAA", a, "ZZZ")
	AddCallback(set, "BBB", b, "AAA")

	type FuncType func() string
	for _, cb := range set.CopyCallbacks() {
		if f, ok := cb.callback.(func() string); ok {
			if cb.name != FuncType(f)() {
				t.Errorf("expected %s, result: %s", cb.name, FuncType(f)())
			} else {
				DeleteCallback(set, cb.name)
			}

		}
	}

	if DeleteCallback(nil, "") != nil {
		t.Errorf("delete the nil set should not be OK")
	}
	DeleteCallback(set, "ABC")

	n := len(set.GetCallbacks())
	if n > 0 {
		t.Errorf("expected: 0, result:%d", n)
	}
}
