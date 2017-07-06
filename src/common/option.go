package common

import "strconv"

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
	*i = Uint16Value(v)
	return err
}

func (i *Uint16Value) Get() interface{} { return uint16(*i) }

func (i *Uint16Value) String() string { return strconv.FormatUint(uint64(*i), 10) }
