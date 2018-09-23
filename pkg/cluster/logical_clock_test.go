package cluster

import (
	"testing"
)

func TestLogicalClock(t *testing.T) {
	l := &logicalClock{}

	if l.Time() != 0 {
		t.Fatalf("bad time value")
	}

	if l.Increase() != 1 {
		t.Fatalf("bad time value")
	}

	if l.Time() != 1 {
		t.Fatalf("bad time value")
	}

	l.Update(41)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}

	l.Update(41)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}

	l.Update(30)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}
}
