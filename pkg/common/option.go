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
	"regexp"
	"strconv"
)

// Uint8Value is the type of uint8
type Uint8Value uint8

// NewUint8Value creates a uint8 value
func NewUint8Value(val uint8, p *uint8) *Uint8Value {
	if p == nil {
		p = new(uint8)
	}
	*p = val
	return (*Uint8Value)(p)
}

// Set convert a string to a uint8 value
func (i *Uint8Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 8)
	if err != nil {
		return err
	}

	*i = Uint8Value(v)
	return nil
}

// Get gets the unit8 value
func (i *Uint8Value) Get() interface{} { return uint8(*i) }

// String converts the unit8 to string
func (i *Uint8Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

////

// Uint16Value is unit16 type
type Uint16Value uint16

// NewUint16Value creates a uint16 value
func NewUint16Value(val uint16, p *uint16) *Uint16Value {
	if p == nil {
		p = new(uint16)
	}
	*p = val
	return (*Uint16Value)(p)
}

// Set converts a string to a uint16 value
func (i *Uint16Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return err
	}

	*i = Uint16Value(v)
	return nil
}

// Get uint16 value
func (i *Uint16Value) Get() interface{} { return uint16(*i) }

// String converts a uint16 value to a string
func (i *Uint16Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

////

// Uint32Value is uint32 type
type Uint32Value uint32

// NewUint32Value creates uint32 value
func NewUint32Value(val uint32, p *uint32) *Uint32Value {
	if p == nil {
		p = new(uint32)
	}
	*p = val
	return (*Uint32Value)(p)
}

// Set converts string to a uint32 value
func (i *Uint32Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return err
	}

	*i = Uint32Value(v)
	return nil
}

// Get gets the uint32 value
func (i *Uint32Value) Get() interface{} { return uint32(*i) }

// String converts the uint32 value to string
func (i *Uint32Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

////

// Uint64RangeValue is uint value with range
type Uint64RangeValue struct {
	v        *uint64
	min, max uint64
}

// NewUint64RangeValue creates a uint64 value with [min, max] range
func NewUint64RangeValue(val uint64, p *uint64, min, max uint64) *Uint64RangeValue {
	if p == nil {
		p = new(uint64)
	}
	*p = val

	return &Uint64RangeValue{
		v:   p,
		min: min,
		max: max,
	}
}

// Set converts a string to a uint64 value with checking the min and max
func (i *Uint64RangeValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return err
	}

	if v < i.min || v > i.max {
		return fmt.Errorf("value out of range [%d, %d]", i.min, i.max)
	}

	*i.v = v
	return nil
}

// Get gets the uint64 value
func (i *Uint64RangeValue) Get() interface{} { return *i.v }

// String converts the uint64 value to string
func (i *Uint64RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	}
	return strconv.FormatUint(*i.v, 10)

}

////

// Uint32RangeValue is the uint32 type with range
type Uint32RangeValue struct {
	v        *uint32
	min, max uint32
}

// NewUint32RangeValue creates the uint32 value with [min, max] range
func NewUint32RangeValue(val uint32, p *uint32, min, max uint32) *Uint32RangeValue {
	if p == nil {
		p = new(uint32)
	}
	*p = val

	return &Uint32RangeValue{
		v:   p,
		min: min,
		max: max,
	}
}

// Set converts a string to uint32 with checking the range
func (i *Uint32RangeValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return err
	}

	v1 := uint32(v)

	if v1 < i.min || v1 > i.max {
		return fmt.Errorf("value out of range [%d, %d]", i.min, i.max)
	}

	*i.v = v1
	return nil
}

// Get returns the uint32 value
func (i *Uint32RangeValue) Get() interface{} { return *i.v }

// String converts the uint32 value to string
func (i *Uint32RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	}
	return strconv.FormatUint(uint64(*i.v), 10)

}

////

// Uint16RangeValue is type of uint6 with range
type Uint16RangeValue struct {
	v        *uint16
	min, max uint16
}

// NewUint16RangeValue creates uint16 range with [min, max] range
func NewUint16RangeValue(val uint16, p *uint16, min, max uint16) *Uint16RangeValue {
	if p == nil {
		p = new(uint16)
	}
	*p = val

	return &Uint16RangeValue{
		v:   p,
		min: min,
		max: max,
	}
}

// Set converts string to uint16 value with checking the range
func (i *Uint16RangeValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return err
	}

	v1 := uint16(v)

	if v1 < i.min || v1 > i.max {
		return fmt.Errorf("value out of range [%d, %d]", i.min, i.max)
	}

	*i.v = v1
	return nil
}

//Get return uint16 value
func (i *Uint16RangeValue) Get() interface{} { return *i.v }

//String coverts the uint16 value to string
func (i *Uint16RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	}
	return strconv.FormatUint(uint64(*i.v), 10)

}

////

//StringRegexValue is a string with regular expression
type StringRegexValue struct {
	s  *string
	re *regexp.Regexp
}

// NewStringRegexValue create a StringRegexValue
func NewStringRegexValue(val string, p *string, r *regexp.Regexp) *StringRegexValue {
	if p == nil {
		p = new(string)
	}
	*p = val

	return &StringRegexValue{
		s:  p,
		re: r,
	}
}

// Set sets a string with can match the regular expression
func (s *StringRegexValue) Set(val string) error {
	if s.re != nil && !s.re.Match([]byte(val)) {
		return fmt.Errorf("invalid pattern, need to match %v", s.re)
	}

	*s.s = val
	return nil
}

// Get returns the string
func (s *StringRegexValue) Get() interface{} { return *s.s }

// String returns the string of StringRegexValue
func (s *StringRegexValue) String() string {
	if s.s == nil {
		return ""
	}

	return *s.s
}
