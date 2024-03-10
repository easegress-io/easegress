/*
 * Copyright (c) 2017, The Easegress Authors
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

package v

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestGetSchema(t *testing.T) {
	assert := assert.New(t)

	strType := reflect.TypeOf("")
	schema, err := GetSchema(strType)
	assert.Nil(err)
	assert.Equal("string", schema.Type)
}
