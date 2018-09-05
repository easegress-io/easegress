package gateway

import (
	"reflect"
	"testing"
)

func TestAggregateNames(t *testing.T) {
	var r1 StatResult
	r1 = &ResultStatIndicatorNames{
		Names: []string{"pipeline-001", "pipeline-002"},
	}
	other := make(map[string]StatResult)

	var r2 StatResult
	r2 = &ResultStatIndicatorNames{
		Names: []string{"pipeline-003", "pipeline-004"},
	}
	other["r2"] = r2

	expected := &ResultStatIndicatorNames{
		Names: []string{"pipeline-001", "pipeline-002", "pipeline-003", "pipeline-004"},
	}

	real, err := r1.Aggregate(false, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(expected, real) {
		t.Fatalf("expected: %+v, but real: %+v", expected, real)
	}
}

func TestAggregateTaskValue(t *testing.T) {
	var r1 StatResult
	var r2 StatResult
	r1 = &ResultStatIndicatorValue{
		MemberName: "m1",
		Name:       "EXECUTION_COUNT_LAST_1MIN_ALL",
		Value:      int64(10),
	}
	r2 = &ResultStatIndicatorValue{
		MemberName: "m2",
		Name:       "EXECUTION_COUNT_LAST_1MIN_ALL",
		Value:      int64(12),
	}

	expected := &AggregatedResultStatIndicatorValue{
		Value: map[string]*AggregatedValue{
			"EXECUTION_COUNT_LAST_1MIN_ALL": {
				AggregatedResult: int64(22),
				AggregatorName:   "numeric_sum",
			},
		},
	}

	other := make(map[string]StatResult)
	other["m2"] = r2

	real, err := r1.Aggregate(false, other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(expected, real) {
		t.Fatalf("expected: %+v, but real: %+v", expected, real)
	}
}
