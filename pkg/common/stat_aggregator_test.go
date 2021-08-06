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
	"testing"
)

type errMapper struct {
	errs  []error
	index int
}

func newErrMapper(errs []error) *errMapper { return &errMapper{errs: errs} }

func (em *errMapper) mapNext(t *testing.T, err error) {
	if em.index >= len(em.errs) {
		t.Fatalf("BUG: map beyond existent entries")
	}

	var (
		want = "<nil>"
		got  = "<nil>"
	)

	switch {
	case em.errs[em.index] == nil && err == nil:
		em.index++
		return
	case em.errs[em.index] != nil && err == nil:
		want = em.errs[em.index].Error()
	case em.errs[em.index] == nil && err != nil:
		got = err.Error()
	case em.errs[em.index] != nil && err != nil:
		want = em.errs[em.index].Error()
		got = err.Error()
		if want == got {
			em.index++
			return
		}
	}

	t.Fatalf("map failed in [%d]:\nwant: %s\ngot : %s", em.index, want, got)
}

func mockUnsupportedKindError(got interface{}) error {
	return fmt.Errorf("unsupported kind %s", reflect.ValueOf(got).Kind())
}

func mockWantXGotYKindError(want numericKind, got interface{}) error {
	return fmt.Errorf("want kind %s, got %s",
		want,
		reflect.ValueOf(got).Kind())
}

func TestNumericMaxAggregatorInt(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(""),
		nil,
		nil,
		mockWantXGotYKindError(intNum, uint(0)),
		nil,
		nil,
		mockWantXGotYKindError(intNum, float64(0.0)),
	})

	a := &NumericMaxAggregator{}
	em.mapNext(t, a.Aggregate(""))
	em.mapNext(t, a.Aggregate(int(2)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(uint(10)))
	em.mapNext(t, a.Aggregate(int(10)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(float64(2.0)))

	wantResult := int64(10)
	gotResult := a.Result().(int64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}
	name := "numeric_max"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericMaxAggregatorUint(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(""),
		nil,
		nil,
	})

	a := &NumericMaxAggregator{}
	em.mapNext(t, a.Aggregate(""))
	em.mapNext(t, a.Aggregate(uint(2)))
	em.mapNext(t, a.Aggregate(uint(3)))

	wantResult := uint64(3)
	gotResult := a.Result().(uint64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}
	name := "numeric_max"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericMaxAggregatorFloat(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(""),
		nil,
		nil,
		mockWantXGotYKindError(floatNum, uint(0)),
	})

	a := &NumericMaxAggregator{}
	em.mapNext(t, a.Aggregate(""))
	em.mapNext(t, a.Aggregate(float64(-2)))
	em.mapNext(t, a.Aggregate(float64(100)))
	em.mapNext(t, a.Aggregate(uint(10)))

	wantResult := float64(100)
	gotResult := a.Result().(float64)
	if gotResult != wantResult {
		t.Fatalf("want %+v, got %+v", wantResult, gotResult)
	}
	name := "numeric_max"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericMinAggregatorFloat(t *testing.T) {
	em := newErrMapper([]error{
		nil,
		mockWantXGotYKindError(floatNum, int(-2)),
		mockWantXGotYKindError(floatNum, uint(10)),
		nil,
		nil,
		mockUnsupportedKindError([]float64{-2.3}),
	})

	a := &NumericMinAggregator{}
	em.mapNext(t, a.Aggregate(float64(2.3)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(uint(10)))
	em.mapNext(t, a.Aggregate(float32(-100)))
	em.mapNext(t, a.Aggregate(float64(100)))
	em.mapNext(t, a.Aggregate([]float64{-2.3}))

	wantResult := float64(-100)
	gotResult := a.Result().(float64)
	if gotResult != wantResult {
		t.Fatalf("want %v, got %v", wantResult, gotResult)
	}

	name := "numeric_min"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericMinAggregatorInt(t *testing.T) {
	em := newErrMapper([]error{
		nil,
		mockWantXGotYKindError(intNum, uint(3)),
		mockWantXGotYKindError(intNum, uint(10)),
		nil,
	})

	a := &NumericMinAggregator{}
	em.mapNext(t, a.Aggregate(int(23)))
	em.mapNext(t, a.Aggregate(uint(3)))
	em.mapNext(t, a.Aggregate(uint(10)))
	em.mapNext(t, a.Aggregate(int(-100)))

	wantResult := int64(-100)
	gotResult := a.Result().(int64)
	if gotResult != wantResult {
		t.Fatalf("want %v, got %v", wantResult, gotResult)
	}

	name := "numeric_min"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericMinAggregatorUint(t *testing.T) {
	em := newErrMapper([]error{
		nil,
		mockWantXGotYKindError(uintNum, int(3)),
		mockWantXGotYKindError(uintNum, int(10)),
		nil,
	})

	a := &NumericMinAggregator{}
	em.mapNext(t, a.Aggregate(uint(25)))
	em.mapNext(t, a.Aggregate(int(3)))
	em.mapNext(t, a.Aggregate(int(10)))
	em.mapNext(t, a.Aggregate(uint(23)))

	wantResult := uint64(23)
	gotResult := a.Result().(uint64)
	if gotResult != wantResult {
		t.Fatalf("want %v, got %v", wantResult, gotResult)
	}

	name := "numeric_min"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericSumAggregatorUint(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(map[int]int{0: 1}),
		nil,
		mockWantXGotYKindError(uintNum, int(-2)),
		nil,
		mockWantXGotYKindError(uintNum, float32(10.1)),
		nil,
	})

	a := &NumericSumAggregator{}
	em.mapNext(t, a.Aggregate(map[int]int{0: 1}))
	em.mapNext(t, a.Aggregate(uint(2)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(uint(10)))
	em.mapNext(t, a.Aggregate(float32(10.1)))
	em.mapNext(t, a.Aggregate(uint(0)))

	wantResult := uint64(12)
	gotResult := a.Result().(uint64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}

	name := "numeric_sum"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericSumAggregatorInt(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(map[int]int{0: 1}),
		nil,
		mockWantXGotYKindError(intNum, uint(2)),
		nil,
		mockWantXGotYKindError(intNum, float32(10.1)),
	})

	a := &NumericSumAggregator{}
	em.mapNext(t, a.Aggregate(map[int]int{0: 1}))
	em.mapNext(t, a.Aggregate(int(2)))
	em.mapNext(t, a.Aggregate(uint(2)))
	em.mapNext(t, a.Aggregate(int(10)))
	em.mapNext(t, a.Aggregate(float32(10.1)))

	wantResult := int64(12)
	gotResult := a.Result().(int64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}

	name := "numeric_sum"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericSumAggregatorFloat(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(map[int]int{0: 1}),
		nil,
		mockWantXGotYKindError(floatNum, int(-2)),
		nil,
	})

	a := &NumericSumAggregator{}
	em.mapNext(t, a.Aggregate(map[int]int{0: 1}))
	em.mapNext(t, a.Aggregate(float64(2)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(float64(10)))

	wantResult := float64(12)
	gotResult := a.Result().(float64)
	if gotResult != wantResult {
		t.Fatalf("want %+v, got %+v", wantResult, gotResult)
	}

	name := "numeric_sum"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericAvgAggregatorInt(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(make(chan int)),
		nil,
		nil,
		mockWantXGotYKindError(intNum, float32(-10.0)),
		mockWantXGotYKindError(intNum, uint(100)),
		nil,
		nil,
		nil,
	})

	a := &NumericAvgAggregator{}
	em.mapNext(t, a.Aggregate(make(chan int)))
	em.mapNext(t, a.Aggregate(int(2)))
	em.mapNext(t, a.Aggregate(int(-2)))
	em.mapNext(t, a.Aggregate(float32(-10.0)))
	em.mapNext(t, a.Aggregate(uint(100)))
	em.mapNext(t, a.Aggregate(int(-10)))
	em.mapNext(t, a.Aggregate(int(0)))
	em.mapNext(t, a.Aggregate(int(110)))

	wantResult := int64(20)
	gotResult := a.Result().(int64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}

	name := "numeric_average"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericAvgAggregatorFloat(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(make(chan int)),
		nil,
		nil,
	})

	a := &NumericAvgAggregator{}
	em.mapNext(t, a.Aggregate(make(chan int)))
	em.mapNext(t, a.Aggregate(float64(2)))
	em.mapNext(t, a.Aggregate(float64(-2)))

	wantResult := float64(0)
	gotResult := a.Result().(float64)
	if gotResult != wantResult {
		t.Fatalf("want %+v, got %+v", wantResult, gotResult)
	}

	name := "numeric_average"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestNumericAvgAggregatorUint(t *testing.T) {
	em := newErrMapper([]error{
		mockUnsupportedKindError(make(chan int)),
		nil,
		nil,
		mockWantXGotYKindError(uintNum, float32(-10.0)),
		mockWantXGotYKindError(uintNum, int(100)),
		nil,
		nil,
		nil,
	})

	a := &NumericAvgAggregator{}
	em.mapNext(t, a.Aggregate(make(chan int)))
	em.mapNext(t, a.Aggregate(uint(2)))
	em.mapNext(t, a.Aggregate(uint(4)))
	em.mapNext(t, a.Aggregate(float32(-10.0)))
	em.mapNext(t, a.Aggregate(int(100)))
	em.mapNext(t, a.Aggregate(uint(4)))
	em.mapNext(t, a.Aggregate(uint(2)))

	wantResult := uint64(3)
	gotResult := a.Result().(uint64)
	if gotResult != wantResult {
		t.Fatalf("want %d, got %d", wantResult, gotResult)
	}

	name := "numeric_average"
	if a.String() != name {
		t.Errorf("want %s, got %s", "numeric_max", name)
	}
}

func TestEdgeCases(t *testing.T) {
	// aggregate nil
	a1 := &NumericMaxAggregator{}
	if err := a1.Aggregate(nil); err != nil {
		t.Errorf("return nil if aggregate nil")
	}
	a2 := &NumericSumAggregator{}
	if err := a2.Aggregate(nil); err != nil {
		t.Errorf("return nil if aggregate nil")
	}
	if res := a2.Result(); res != nil {
		t.Errorf("return nil for nil res")
	}
	a3 := &NumericAvgAggregator{}
	if err := a3.Aggregate(nil); err != nil {
		t.Errorf("return nil if aggregate nil")
	}
	a4 := &NumericMinAggregator{}
	if err := a4.Aggregate(nil); err != nil {
		t.Errorf("return nil if aggregate nil")
	}

	// nk
	var nk numericKind
	if nk.String() != "invalid" {
		t.Errorf("invalid numericKind error")
	}
	nk = 4
	if nk.String() != "unknown numericKind" {
		t.Errorf("unknown numericKind error")
	}

}
