package v

import (
	"encoding/json"
	"fmt"
	"reflect"

	yamljsontool "github.com/ghodss/yaml"
	genjs "github.com/megaease/jsonschema"
	loadjs "github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v2"
)

type (
	schemaMeta struct {
		schema *loadjs.Schema

		jsonFormat []byte
		yamlFormat []byte
	}
)

var (
	globalReflector = &genjs.Reflector{
		RequiredFromJSONSchemaTags: true,
	}
	schemaMetas = map[reflect.Type]*schemaMeta{}
)

// Validate validates by json schema rules.
func Validate(v interface{}) *ValidateRecorder {
	vr := &ValidateRecorder{}

	sm, err := getSchemaMeta(v)
	if err != nil {
		vr.recordSystem(fmt.Errorf("get schema meta for %T failed: %v", v, err))
		return vr
	}

	yamlBuff, err := yaml.Marshal(v)
	if err != nil {
		vr.recordSystem(fmt.Errorf("marshal %#v to yaml failed: %v", v, err))
		return vr
	}

	jsonBuff, err := yamljsontool.YAMLToJSON(yamlBuff)
	if err != nil {
		vr.recordSystem(fmt.Errorf("transform %s to json failed: %v", yamlBuff, err))
		return vr
	}

	docLoader := loadjs.NewBytesLoader(jsonBuff)

	result, err := sm.schema.Validate(docLoader)
	vr.recordJSONSchema(result)

	val := reflect.ValueOf(v)
	traverseGo(&val, nil, vr.recordFormat)
	traverseGo(&val, nil, vr.recordGeneral)

	return vr
}

func getSchemaMeta(v interface{}) (*schemaMeta, error) {
	t := reflect.TypeOf(v)
	sm, exists := schemaMetas[t]
	if exists {
		return sm, nil
	}

	var err error

	sm = &schemaMeta{}
	schema := globalReflector.ReflectFromType(t)
	if _, ok := getFormatFunc(schema.Format); !ok {
		return nil, fmt.Errorf("%T got unsupported format: %s", v, schema.Format)
	}
	for _, t := range schema.Definitions {
		if _, ok := getFormatFunc(t.Format); !ok {
			return nil, fmt.Errorf("%T got unsupported format: %s", v, t.Format)
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
// 1. It traverses fields of the embeded struct.
// 2. It does not traverse unexported subfields of the struct.
// 3. It pass nil to the argument StructField when it's not a struct field.
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
			// unexported
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
