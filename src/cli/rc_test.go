package cli

import (
	"fmt"
	"testing"
)

func TestSetLocalOperationSequence(t *testing.T) {
	err := setLocalOperationSequence("test-group", 4)
	if err != nil {
		t.Errorf("setLocalOperationSequence failed:", err)
	}
}

func TestGetLocalOperationSequence(t *testing.T) {
	seq, err := getLocalOperationSequence("test-group")
	if err != nil {
		t.Errorf("setLocalOperationSequence failed:", err)
	}
	fmt.Println("local sequence:", seq)
}
