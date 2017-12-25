package common

import (
	"fmt"
	"regexp"
	"strconv"
)

type Uint16Value uint16

func NewUint16Value(val uint16, p *uint16) *Uint16Value {
	if p == nil {
		p = new(uint16)
	}
	*p = val
	return (*Uint16Value)(p)
}

func (i *Uint16Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return err
	}

	*i = Uint16Value(v)
	return nil
}

func (i *Uint16Value) Get() interface{} { return uint16(*i) }

func (i *Uint16Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

////

type Uint32Value uint32

func NewUint32Value(val uint32, p *uint32) *Uint32Value {
	if p == nil {
		p = new(uint32)
	}
	*p = val
	return (*Uint32Value)(p)
}

func (i *Uint32Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return err
	}

	*i = Uint32Value(v)
	return nil
}

func (i *Uint32Value) Get() interface{} { return uint32(*i) }

func (i *Uint32Value) String() string { return strconv.FormatUint(uint64(*i), 10) }

////

type Uint64RangeValue struct {
	v        *uint64
	min, max uint64
}

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

func (i *Uint64RangeValue) Get() interface{} { return *i.v }

func (i *Uint64RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	} else {
		return strconv.FormatUint(*i.v, 10)
	}
}

////

type Uint32RangeValue struct {
	v        *uint32
	min, max uint32
}

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

func (i *Uint32RangeValue) Get() interface{} { return *i.v }

func (i *Uint32RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	} else {
		return strconv.FormatUint(uint64(*i.v), 10)
	}
}

////

type Uint16RangeValue struct {
	v        *uint16
	min, max uint16
}

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

func (i *Uint16RangeValue) Get() interface{} { return *i.v }

func (i *Uint16RangeValue) String() string {
	if i.v == nil {
		return strconv.FormatUint(0, 10) // zero value
	} else {
		return strconv.FormatUint(uint64(*i.v), 10)
	}
}

////

type StringRegexValue struct {
	s  *string
	re *regexp.Regexp
}

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

func (s *StringRegexValue) Set(val string) error {
	if s.re != nil && !s.re.Match([]byte(val)) {
		return fmt.Errorf("invalid pattern, need to match %v", s.re)
	}

	*s.s = val
	return nil
}

func (s *StringRegexValue) Get() interface{} { return *s.s }

func (s *StringRegexValue) String() string {
	if s.s == nil {
		return ""
	}

	return *s.s
}
