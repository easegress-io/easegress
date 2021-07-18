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
	"regexp"
	"testing"
)

func TestUint8(t *testing.T) {
	var u8 uint8
	u := NewUint8Value(128, &u8)
	if u8 != 128 {
		t.Errorf("expected %d, result %d", 128, u8)
	}

	str := "100"
	u8 = 100
	u.Set(str)
	if u.Get() != uint8(u8) {
		t.Errorf("expected %d, result %d", u8, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}
}

func TestUint16(t *testing.T) {
	var u16 uint16
	u := NewUint16Value(512, &u16)
	if u16 != 512 {
		t.Errorf("expected %d, result %d", 512, u16)
	}

	str := "500"
	u16 = 600
	u.Set(str)
	if u.Get() != u16 {
		t.Errorf("expected %d, result %d", u16, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}
}

func TestUint32(t *testing.T) {
	var u32 uint32
	u := NewUint32Value(512, &u32)
	if u32 != 512 {
		t.Errorf("expected %d, result %d", 512, u32)
	}

	str := "32000"
	u32 = 9990
	u.Set(str)
	if u.Get() != u32 {
		t.Errorf("expected %d, result %d", u32, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}
}

func TestUint64Range(t *testing.T) {
	var u64 uint64
	u := NewUint64RangeValue(512, &u64, 1, 1000000000)
	if u64 != 512 {
		t.Errorf("expected %d, result %d", 512, u64)
	}

	str := "32000"
	u64 = 9990
	u.Set(str)
	if u.Get() != u64 {
		t.Errorf("expected %d, result %d", u64, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}
	str = "1000000001"
	if err:= u.Set(str); err == nil {
		t.Errorf("it should be wrong, because the %s out of range", str)
	}
}

func TestUint32Range(t *testing.T) {
	var u32 uint32
	u := NewUint32RangeValue(512, &u32, 1, 100000)
	if u32 != 512 {
		t.Errorf("expected %d, result %d", 512, u32)
	}

	str := "32000"
	u32 = 9990
	u.Set(str)
	if u.Get() != u32 {
		t.Errorf("expected %d, result %d", u32, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}
	str = "100001"
	if err:= u.Set(str); err == nil {
		t.Errorf("it should be wrong, because the %s out of range", str)
	}
}

func TestUint16Range(t *testing.T) {
	var u16 uint16
	u := NewUint16RangeValue(512, &u16, 1, 1000)
	if u16 != 512 {
		t.Errorf("expected %d, result %d", 512, u16)
	}

	str := "300"
	u16 = 999
	u.Set(str)
	if u.Get() != u16 {
		t.Errorf("expected %d, result %d", u16, u.Get())
	}
	if u.String() != str {
		t.Errorf("expected %s, result %s", str, u.String())
	}

	str = "1024"
	if err:= u.Set(str); err == nil {
		t.Errorf("it should be wrong, because the %s out of range", str)
	}
}

func TestStringRegexValue(t *testing.T) {
	re := regexp.MustCompile(`[0-9]+`)
	var str string
	s := NewStringRegexValue("1234", &str, re)

	if s.re != re || *(s.s) != "1234" {
		t.Errorf("expected %s, result %v", str, s)
	}

	v := "5678"
	if err := s.Set(v); err !=nil {
		t.Errorf("%v", err)
	}

	if err := s.Set("abcd"); err ==nil {
		t.Errorf("invalid pattern, should be return error ")
	}

	if s.Get() != v || s.String() != v {
		t.Errorf("expected: %s, result %s", v, s.Get())
	}
	
}
