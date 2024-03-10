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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFieldJSONName(t *testing.T) {
	assert := assert.New(t)

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	u := User{
		ID:   1,
		Name: "Alice",
	}

	val := reflect.ValueOf(u)
	typeOfT := val.Type()
	for i := 0; i < val.NumField(); i++ {
		fieldType := typeOfT.Field(i)
		// ID -> id, Name -> name
		assert.Equal(strings.ToLower(fieldType.Name), getFieldJSONName(&fieldType))
	}
}

func TestRequiredFromField(t *testing.T) {
	assert := assert.New(t)

	type User struct {
		ID   int    `json:"id,omitempty"`
		Name string `json:"name,omitempty" jsonschema:"-"`
		Addr string `json:"address,omitempty" jsonschema:"required"`
	}

	u := User{
		ID:   1,
		Name: "Alice",
	}

	val := reflect.ValueOf(u)
	typeOfT := val.Type()

	idField, _ := typeOfT.FieldByName("ID")
	assert.True(IsOmitemptyField(&idField))

	nameField, _ := typeOfT.FieldByName("Name")
	assert.True(IsOmitemptyField(&nameField))

	addrField, _ := typeOfT.FieldByName("Addr")
	assert.True(IsOmitemptyField(&addrField))
}

func TestValidateRecorder(t *testing.T) {
	assert := assert.New(t)

	type TestStruct struct {
		ID     int    `json:"id"`
		Name   string `json:"name" jsonschema:"-"`
		Addr   string `json:"address" jsonschema:"required"`
		Method string `json:"method" jsonschema:"format=httpmethod"`
	}

	u := TestStruct{
		ID:     1,
		Name:   "Alice",
		Addr:   "123",
		Method: "GET",
	}
	val := reflect.ValueOf(u)
	typeOfT := val.Type()

	vr := &ValidateRecorder{}
	for i := 0; i < val.NumField(); i++ {
		subVal := val.Field(i)
		fieldType := typeOfT.Field(i)
		vr.recordFormat(&subVal, &fieldType)
	}

	sysErr := errors.New("system error")
	vr.recordSystem(sysErr)
	assert.Equal(vr.SystemErr, sysErr.Error())

	assert.Equal(vr.Error(), vr.String())
	assert.False(vr.Valid())
}
