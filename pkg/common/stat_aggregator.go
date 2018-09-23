package common

import (
	"fmt"
	"reflect"
)

type StatAggregator interface {
	Aggregate(interface{}) error
	Result() interface{}
	String() string
}

type numericKind uint

const (
	invalidNum numericKind = iota
	intNum
	uintNum
	floatNum
)

func (nk numericKind) String() string {
	switch nk {
	case invalidNum:
		return "invalid"
	case intNum:
		return "int[8|16|32|64]"
	case uintNum:
		return "uint[8|16|32|64]"
	case floatNum:
		return "float[32|64]"
	}
	return "unknown numericKind"
}

func unifyNumericKind(num interface{}) (interface{}, numericKind) {
	switch reflect.ValueOf(num).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(num).Int(), intNum
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(num).Uint(), uintNum
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(num).Float(), floatNum
	default:
		return num, invalidNum
	}
}

type NumericMaxAggregator struct {
	max interface{}
	nk  numericKind
}

func (a *NumericMaxAggregator) String() string {
	return "numeric_max"
}

func (a *NumericMaxAggregator) Aggregate(num interface{}) error {
	if num == nil {
		return nil
	}

	kind := reflect.ValueOf(num).Kind()
	value, nk := unifyNumericKind(num)
	if nk == invalidNum {
		return fmt.Errorf("unsupported kind %s", kind)
	}

	if a.max == nil {
		a.max, a.nk = value, nk
		return nil
	}

	if a.nk != nk {
		return fmt.Errorf("want kind %s, got %s", a.nk, kind)
	}

	switch a.nk {
	case intNum:
		if a.max.(int64) < value.(int64) {
			a.max = value
		}
	case uintNum:
		if a.max.(uint64) < value.(uint64) {
			a.max = value
		}
	case floatNum:
		if a.max.(float64) < value.(float64) {
			a.max = value
		}
	default:
		return fmt.Errorf("BUG: unsuppoorted numeric kind %s", a.nk)
	}

	return nil
}

func (a *NumericMaxAggregator) Result() interface{} {
	return a.max
}

type NumericMinAggregator struct {
	min interface{}
	nk  numericKind
}

func (a *NumericMinAggregator) String() string {
	return "numeric_min"
}

func (a *NumericMinAggregator) Aggregate(num interface{}) error {
	if num == nil {
		return nil
	}

	kind := reflect.ValueOf(num).Kind()
	value, nk := unifyNumericKind(num)
	if nk == invalidNum {
		return fmt.Errorf("unsupported kind %s", kind)
	}

	if a.min == nil {
		a.min, a.nk = value, nk
		return nil
	}

	if a.nk != nk {
		return fmt.Errorf("want kind %s, got %s", a.nk, kind)
	}

	switch a.nk {
	case intNum:
		if a.min.(int64) > value.(int64) {
			a.min = value
		}
	case uintNum:
		if a.min.(uint64) > value.(uint64) {
			a.min = value
		}
	case floatNum:
		if a.min.(float64) > value.(float64) {
			a.min = value
		}
	default:
		return fmt.Errorf("BUG: unsuppoorted numeric kind %s", a.nk)
	}

	return nil
}

func (a *NumericMinAggregator) Result() interface{} {
	return a.min
}

type NumericSumAggregator struct {
	sum interface{}
	nk  numericKind
}

func (a *NumericSumAggregator) String() string {
	return "numeric_sum"
}

func (a *NumericSumAggregator) Aggregate(num interface{}) error {
	if num == nil {
		return nil
	}

	kind := reflect.ValueOf(num).Kind()
	value, nk := unifyNumericKind(num)
	if nk == invalidNum {
		return fmt.Errorf("unsupported kind %s", kind)
	}

	if a.sum == nil {
		a.sum, a.nk = value, nk
		return nil
	}

	if a.nk != nk {
		return fmt.Errorf("want kind %s, got %s", a.nk, kind)
	}

	switch a.nk {
	case intNum:
		a.sum = a.sum.(int64) + value.(int64)
	case uintNum:
		a.sum = a.sum.(uint64) + value.(uint64)
	case floatNum:
		a.sum = a.sum.(float64) + value.(float64)
	default:
		return fmt.Errorf("BUG: unsuppoorted numeric kind %s", a.nk)
	}

	return nil
}

func (a *NumericSumAggregator) Result() interface{} {
	return a.sum
}

type NumericAvgAggregator struct {
	NumericSumAggregator
	count int64
}

func (a *NumericAvgAggregator) String() string {
	return "numeric_average"
}

func (a *NumericAvgAggregator) Aggregate(num interface{}) error {
	if num == nil {
		return nil
	}

	err := a.NumericSumAggregator.Aggregate(num)
	if err != nil {
		return err
	}

	a.count++

	return nil
}

func (a *NumericAvgAggregator) Result() interface{} {
	if a.NumericSumAggregator.Result() == nil {
		return nil
	}

	switch a.nk {
	case intNum:
		return a.NumericSumAggregator.Result().(int64) / int64(a.count)
	case uintNum:
		return a.NumericSumAggregator.Result().(uint64) / uint64(a.count)
	case floatNum:
		return a.NumericSumAggregator.Result().(float64) / float64(a.count)
	default:
		return nil
	}
}
