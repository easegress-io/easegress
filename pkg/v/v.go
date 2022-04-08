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

package v

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	genjs "github.com/alecthomas/jsonschema"
	yamljsontool "github.com/ghodss/yaml"
	loadjs "github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/jsontool"
)

type (
	schemaMeta struct {
		schema *loadjs.Schema

		jsonFormat []byte
		yamlFormat []byte
	}
	// ContentValidator is used to validate by data content.
	ContentValidator interface {
		Validate([]byte) error
	}
)

var (
	reflector = &genjs.Reflector{
		AllowAdditionalProperties:  true,
		RequiredFromJSONSchemaTags: true,
		PreferYAMLSchema:           true,
		DoNotReference:             true,
		ExpandedStruct:             true,

		// NOTE: FullyQualifyTypeNames setting true will generate
		// "$ref": "#/definitions/github.com/megaease/easegress/pkg/supervisor.MetaSpec",
		// "definitions": {
		//   "github.com/megaease/easegress/pkg/supervisor.MetaSpec": {
		//      ...
		//   }
		// }
		// FIXME if necessary:
		// The $ref can't find it because the slash means a level in json schema.
		// We can fix it by replace all slashes by `.` or `-`, etc in github.com/alecthomas/jsonschema.
	}
	schemaMetasMutex = sync.Mutex{}
	schemaMetas      = map[reflect.Type]*schemaMeta{}
)

// GetSchemaInYAML returns the json schema of t in yaml format.
func GetSchemaInYAML(t reflect.Type) ([]byte, error) {
	sm, err := getSchemaMeta(t)
	if err != nil {
		return nil, err
	}

	return sm.yamlFormat, nil
}

// GetSchemaInJSON return the json schema of t in json format.
func GetSchemaInJSON(t reflect.Type) ([]byte, error) {
	sm, err := getSchemaMeta(t)
	if err != nil {
		return nil, err
	}

	return sm.jsonFormat, nil
}

// Validate validates by json schema rules, custom formats and general methods.
func Validate(v interface{}) *ValidateRecorder {
	vr := &ValidateRecorder{}

	if v == nil {
		vr.recordSystem(fmt.Errorf("nil value"))
	}

	yamlBuff, err := yaml.Marshal(v)
	if err != nil {
		vr.recordSystem(fmt.Errorf("marshal %#v to yaml string failed: %v", v, err))
		return vr
	}

	sm, err := getSchemaMeta(reflect.TypeOf(v))
	if err != nil {
		vr.recordSystem(fmt.Errorf("get schema meta for %T failed: %v", v, err))
		return vr
	}

	jsonBuff, err := yamljsontool.YAMLToJSON(yamlBuff)
	if err != nil {
		vr.recordSystem(fmt.Errorf("transform %s to json failed: %v", yamlBuff, err))
		return vr
	}

	trimJSONBuff, err := jsontool.TrimNull(jsonBuff)
	if err != nil {
		vr.recordSystem(fmt.Errorf("trim null from %s failed: %v", jsonBuff, err))
		return vr
	}

	docLoader := loadjs.NewBytesLoader(trimJSONBuff)
	result, err := sm.schema.Validate(docLoader)
	if err != nil {
		logger.Errorf("BUG: invalid schema: %v", err)
	}
	vr.recordJSONSchema(result)

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
	schema := reflector.ReflectFromType(t)
	if _, ok := getFormatFunc(schema.Format); !ok {
		return nil, fmt.Errorf("%v got unsupported format: %s", t, schema.Format)
	}
	for _, definition := range schema.Definitions {
		if _, ok := getFormatFunc(definition.Format); !ok {
			return nil, fmt.Errorf("%v got unsupported format: %s", t, definition.Format)
		}
	}

	sm.jsonFormat, err = json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to json failed: %v", sm.schema, err)
	}

	sm.yamlFormat, err = yamljsontool.JSONToYAML(sm.jsonFormat)
	if err != nil {
		return nil, fmt.Errorf("transform json %s to yaml failed: %v", sm.jsonFormat, err)
	}

	sm.schema, err = loadjs.NewSchema(loadjs.NewBytesLoader(sm.jsonFormat))
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
// 4. It stops when encoutering nil.
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
