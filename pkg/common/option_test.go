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
	"math"
	"regexp"
	"strconv"
	"testing"
)

func TestUint8(t *testing.T) {
	var u8 uint8
	u := NewUint8Value(math.MaxUint8, &u8)
	if u8 != math.MaxUint8 {
		t.Errorf("expected %d, result %d", math.MaxUint8, u8)
	}

	var x uint8 = 100
	u = NewUint8Value(x, nil)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 200
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestUint16(t *testing.T) {
	var u16 uint16
	u := NewUint16Value(math.MaxUint16, &u16)
	if u16 != math.MaxUint16 {
		t.Errorf("expected %d, result %d", math.MaxUint16, u16)
	}

	var x uint16 = 32000
	u = NewUint16Value(x, nil)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 8000
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestUint32(t *testing.T) {
	var u32 uint32
	u := NewUint32Value(math.MaxUint32, &u32)
	if u32 != math.MaxUint32 {
		t.Errorf("expected %d, result %d", math.MaxUint32, u32)
	}

	var x uint32 = 100000000
	u = NewUint32Value(x, nil)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 2000000000
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestUint64Range(t *testing.T) {
	var u64 uint64
	u := NewUint64RangeValue(math.MaxUint64-1, &u64, 1, math.MaxUint64)
	if u64 != math.MaxUint64-1 {
		t.Errorf("expected %d, result %d", uint64(math.MaxUint64-1), u64)
	}

	var x uint64 = 10000000000000
	u = NewUint64RangeValue(x, nil, 1, math.MaxUint64)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 200000000000000
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("0"); err == nil {
		t.Errorf("set the number out of range should not return right!")
	}
	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestUint32Range(t *testing.T) {
	var u32 uint32
	u := NewUint32RangeValue(math.MaxUint32-1, &u32, 1, math.MaxUint32)
	if u32 != math.MaxUint32-1 {
		t.Errorf("expected %d, result %d", uint64(math.MaxUint32-1), u32)
	}

	var x uint32 = 1000000000
	u = NewUint32RangeValue(x, nil, 1, math.MaxUint32)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 2000000000
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("0"); err == nil {
		t.Errorf("set the number out of range should not return right!")
	}
	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestUint16Range(t *testing.T) {
	var u16 uint16
	u := NewUint16RangeValue(math.MaxUint16-1, &u16, 1, math.MaxUint16)
	if u16 != math.MaxUint16-1 {
		t.Errorf("expected %d, result %d", uint64(math.MaxUint16-1), u16)
	}

	var x uint16 = 10000
	u = NewUint16RangeValue(x, nil, 1, math.MaxUint16)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}

	x = 20000
	str := strconv.FormatUint(uint64(x), 10)
	u.Set(str)
	if u.Get() != x {
		t.Errorf("expected %d, result %d", x, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	if err := u.Set("0"); err == nil {
		t.Errorf("set the number out of range should not return right!")
	}
	if err := u.Set("invalid"); err == nil {
		t.Errorf("set the invalid value should not return right!")
	}
}

func TestStringRegexValue(t *testing.T) {
	re := regexp.MustCompile(`[0-9]+`)
	var str string
	s := NewStringRegexValue("1234", &str, re)

	if s.re != re || *(s.s) != "1234" {
		t.Errorf("expected %s, result %v", str, s)
	}

	s = NewStringRegexValue("9999", nil, re)
	if s.re != re || *(s.s) != "9999" {
		t.Errorf("expected %s, result %v", str, s)
	}

	v := "5678"
	if err := s.Set(v); err != nil {
		t.Errorf("%v", err)
	}

	if err := s.Set("abcd"); err == nil {
		t.Errorf("invalid pattern, should be return error ")
	}

	if s.Get() != v || s.String() != v {
		t.Errorf("expected: %s, result %s", v, s.Get())
	}

}
