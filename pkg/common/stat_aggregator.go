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
	"reflect"
)

// StatAggregator is the interface
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

// String return the type of uint
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

// NumericMaxAggregator is the structure with max value
type NumericMaxAggregator struct {
	max interface{}
	nk  numericKind
}

// String return the name
func (a *NumericMaxAggregator) String() string {
	return "numeric_max"
}

// Aggregate records the max of value
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

// Result return the max value
func (a *NumericMaxAggregator) Result() interface{} {
	return a.max
}

// NumericMinAggregator is the structure with min value
type NumericMinAggregator struct {
	min interface{}
	nk  numericKind
}

// String return the name
func (a *NumericMinAggregator) String() string {
	return "numeric_min"
}

// Aggregate records the min value
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

// Result return the min value
func (a *NumericMinAggregator) Result() interface{} {
	return a.min
}

// NumericSumAggregator is the structure with sum value
type NumericSumAggregator struct {
	sum interface{}
	nk  numericKind
}

//String return the name
func (a *NumericSumAggregator) String() string {
	return "numeric_sum"
}

// Aggregate records the sum value
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

//Result return the sum value
func (a *NumericSumAggregator) Result() interface{} {
	return a.sum
}

// NumericAvgAggregator is the structure with average value
type NumericAvgAggregator struct {
	NumericSumAggregator
	count int64
}

// String return the name
func (a *NumericAvgAggregator) String() string {
	return "numeric_average"
}

//Aggregate records the number of value and the sum of values
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

// Result returns the average values
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
