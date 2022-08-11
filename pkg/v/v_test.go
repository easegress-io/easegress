package v

import (
	"testing"
)

type (
	validateOnPointer struct {
		validateCount *int
	}

	validateOnStruct struct {
		validateCount *int
	}
)

func newValidateOnPointer() *validateOnPointer {
	return &validateOnPointer{
		validateCount: new(int),
	}
}

func newValidateOnStruct() *validateOnStruct {
	return &validateOnStruct{
		validateCount: new(int),
	}
}

func (s *validateOnPointer) Validate() error {
	*s.validateCount++
	return nil
}

func (s validateOnStruct) Validate() error {
	*s.validateCount++
	return nil
}

// So the conclusion here would be, if the method Validate is defined on *Spec.
// It will not be called in some situations such as:
// Validate(Spec{}) or Validate(map[string]Spec), etc.
// So please always define Validate on struct level aka. Spec.Validate instead of (*Spec).Validate .

func TestOnPointer(t *testing.T) {
	pointer := newValidateOnPointer()
	Validate(pointer)
	Validate(pointer)

	if *pointer.validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointer.validateCount)
	}

	// NOTE: The struct can't be validated with the method Validate on pointer.
	Validate(*pointer)
	if *pointer.validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointer.validateCount)
	}
}

func TestOnPointers(t *testing.T) {
	pointerValues := map[string]*validateOnPointer{
		"key": newValidateOnPointer(),
	}

	Validate(pointerValues)
	Validate(pointerValues)

	if *pointerValues["key"].validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointerValues["key"].validateCount)
	}

	// NOTE: The value struct can't be validated with the method Validate on pointer.
	structValues := map[string]validateOnPointer{
		"key": *newValidateOnPointer(),
	}

	Validate(structValues)
	Validate(structValues)

	if *structValues["key"].validateCount != 0 {
		t.Errorf("expected validateCount to be 0, got %d", *structValues["key"].validateCount)
	}
}

func TestOnStruct(t *testing.T) {
	pointer := newValidateOnStruct()
	Validate(pointer)
	Validate(pointer)

	if *pointer.validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointer.validateCount)
	}

	Validate(*pointer)
	if *pointer.validateCount != 3 {
		t.Errorf("expected validateCount to be 3, got %d", *pointer.validateCount)
	}
}

func TestOnStructs(t *testing.T) {
	pointerValues := map[string]*validateOnStruct{
		"key": newValidateOnStruct(),
	}

	Validate(pointerValues)
	Validate(pointerValues)

	if *pointerValues["key"].validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointerValues["key"].validateCount)
	}

	structValues := map[string]validateOnStruct{
		"key": *newValidateOnStruct(),
	}

	Validate(structValues)
	Validate(structValues)

	if *pointerValues["key"].validateCount != 2 {
		t.Errorf("expected validateCount to be 2, got %d", *pointerValues["key"].validateCount)
	}
}
