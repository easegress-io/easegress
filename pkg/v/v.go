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

// Package v implements the common validation logic of Easegress.
package v

import (
	"fmt"
	"reflect"
	"sync"

	genjs "github.com/invopop/jsonschema"
	loadjs "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	schemaMeta struct {
		genSchema  *genjs.Schema
		loadSchema *loadjs.Schema

		jsonFormat []byte
	}
)

var (
	reflector = &genjs.Reflector{
		AllowAdditionalProperties:  true,
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             true,

		// NOTE: true value will cause problems.
		// ExpandedStruct:             true,
	}
	schemaMetasMutex = sync.Mutex{}
	schemaMetas      = map[reflect.Type]*schemaMeta{}
)

// GetSchema returns the json schema of t.
func GetSchema(t reflect.Type) (*genjs.Schema, error) {
	sm, err := getSchemaMeta(t)
	if err != nil {
		return nil, err
	}

	return sm.genSchema, nil
}

// Validate validates by json schema rules, custom formats and general methods.
func Validate(v interface{}) *ValidateRecorder {
	vr := &ValidateRecorder{}

	if v == nil {
		vr.recordSystem(fmt.Errorf("nil value"))
	}

	jsonBuff, err := codectool.MarshalJSON(v)
	if err != nil {
		vr.recordSystem(fmt.Errorf("marshal %#v to json failed: %v", v, err))
		return vr
	}
	var rawValue interface{}
	err = codectool.Unmarshal(jsonBuff, &rawValue)
	if err != nil {
		vr.recordSystem(fmt.Errorf("unmarshal json %s failed: %v", jsonBuff, err))
		return vr
	}

	sm, err := getSchemaMeta(reflect.TypeOf(v))
	if err != nil {
		vr.recordSystem(fmt.Errorf("get schema meta for %T failed: %v", v, err))
		return vr
	}

	err = sm.loadSchema.Validate(rawValue)
	vr.recordJSONSchema(err)

	val := reflect.ValueOf(v)
	traverseGo(&val, nil, vr.record)

	return vr
}

func getSchemaMeta(t reflect.Type) (*schemaMeta, error) {
	schemaMetasMutex.Lock()
	defer schemaMetasMutex.Unlock()

	sm, exists := schemaMetas[t]
	if exists {
		return sm, nil
	}

	var err error

	sm = &schemaMeta{}
	sm.genSchema = reflector.ReflectFromType(t)
	if _, ok := getFormatFunc(sm.genSchema.Format); !ok {
		return nil, fmt.Errorf("%v got unsupported format: %s", t, sm.genSchema.Format)
	}
	for _, definition := range sm.genSchema.Definitions {
		if _, ok := getFormatFunc(definition.Format); !ok {
			return nil, fmt.Errorf("%v got unsupported format: %s", t, definition.Format)
		}
	}

	sm.jsonFormat, err = codectool.MarshalJSON(sm.genSchema)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to json failed: %v", sm.loadSchema, err)
	}

	sm.loadSchema, err = loadjs.CompileString(loadjs.Draft2020.String(), string(sm.jsonFormat))
	if err != nil {
		return nil, fmt.Errorf("new schema from %s failed: %v", sm.jsonFormat, err)
	}

	schemaMetas[t] = sm

	return sm, nil
}

// traverseGo recursively traverses the golang data structure with the rules below:
//
// 1. It traverses fields of the embedded struct.
// 2. It does not traverse unexposed subfields of the struct.
// 3. It passes nil to the argument StructField when it's not a struct field.
// 4. It stops when encountering nil.
func traverseGo(val *reflect.Value, field *reflect.StructField, fn func(*reflect.Value, *reflect.StructField)) {
	t := val.Type()

	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Slice, reflect.Ptr:
		if val.IsNil() {
			return
		}
	}

	fn(val, field)

	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			subfield, subval := t.Field(i), val.Field(i)
			// unexposed
			if subfield.PkgPath != "" {
				continue
			}
			if subfield.Type.Kind() == reflect.Ptr && subval.IsNil() {
				continue
			}
			traverseGo(&subval, &subfield, fn)
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			subval := val.Index(i)
			traverseGo(&subval, nil, fn)
		}
	case reflect.Map:
		iter := val.MapRange()
		for iter.Next() {
			k, v := iter.Key(), iter.Value()
			traverseGo(&k, nil, fn)
			traverseGo(&v, nil, fn)
		}
	case reflect.Ptr:
		child := val.Elem()
		traverseGo(&child, nil, fn)
	}
}
