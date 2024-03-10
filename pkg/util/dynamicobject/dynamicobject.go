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

// Package dynamicobject provides a dynamic object.
package dynamicobject

// DynamicObject defines a dynamic object which is a map of string to
// interface{}. The value of this map could also be a dynamic object,
// but in this case, its type must be `map[string]interface{}`, and
// should not be `map[interface{}]interface{}`.
type DynamicObject map[string]interface{}

// UnmarshalYAML implements yaml.Unmarshaler
//
// the type of a DynamicObject field could be `map[interface{}]interface{}`
// if it is unmarshaled from yaml, but some packages, like the standard
// json package could not handle this type, so it must be converted to
// `map[string]interface{}`.
//
// Note there's a bug with this function:
//
//	do := DynamicObject{}
//	yaml.Unmarshal([]byte(`{"a": 1}`), &do)
//	yaml.Unmarshal([]byte(`{"b": 2}`), &do)
//
// the result of above code should be `{"a": 1, "b": 2}`, but it is
// `{"b": 2}`.
func (do *DynamicObject) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}
	if err := unmarshal(&m); err != nil {
		return err
	}

	var convert func(interface{}) interface{}
	convert = func(src interface{}) interface{} {
		switch x := src.(type) {
		case map[interface{}]interface{}:
			x2 := map[string]interface{}{}
			for k, v := range x {
				x2[k.(string)] = convert(v)
			}
			return x2
		case []interface{}:
			x2 := make([]interface{}, len(x))
			for i, v := range x {
				x2[i] = convert(v)
			}
			return x2
		}
		return src
	}

	for k, v := range m {
		m[k] = convert(v)
	}
	*do = m

	return nil
}

// Get gets the value of field 'field'
func (do DynamicObject) Get(field string) interface{} {
	return do[field]
}

// GetString gets the value of field 'field' as string
func (do DynamicObject) GetString(field string) string {
	if v, ok := do[field].(string); ok {
		return v
	}
	return ""
}

// Set set the value of field 'field' to 'value'
func (do *DynamicObject) Set(field string, value interface{}) {
	(*do)[field] = value
}
