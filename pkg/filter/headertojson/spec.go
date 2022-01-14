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

package headertojson

import "strconv"

type (
	// Spec is spec of HeaderToJson
	Spec struct {
		HeaderMap []*HeaderMap `yaml:"headerMap" jsonschema:"required"`
	}

	// Header defines relationship between http header and json
	HeaderMap struct {
		Header string   `yaml:"header" jsonschema:"required"`
		JSON   string   `yaml:"json" jsonschema:"required"`
		Type   JSONType `yaml:"type" jsonschema:"required"`
	}

	JSONType string

	setFn func(key, value string, res map[string]interface{}) error
)

const (
	jsonInt    JSONType = "int"
	jsonFloat  JSONType = "float"
	jsonString JSONType = "string"
	jsonBool   JSONType = "bool"
	jsonNull   JSONType = "null"
)

var jsonTypeMap = map[JSONType]struct{}{
	jsonInt:    {},
	jsonFloat:  {},
	jsonString: {},
	jsonBool:   {},
	jsonNull:   {},
}

var jsonValueMap = map[JSONType]setFn{
	jsonInt: func(key, value string, res map[string]interface{}) error {
		number, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		res[key] = number
		return nil
	},

	jsonFloat: func(key, value string, res map[string]interface{}) error {
		float, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		res[key] = float
		return nil
	},

	jsonString: func(key, value string, res map[string]interface{}) error {
		res[key] = value
		return nil
	},

	jsonBool: func(key, value string, res map[string]interface{}) error {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		res[key] = boolValue
		return nil
	},

	jsonNull: func(key, value string, res map[string]interface{}) error {
		res[key] = nil
		return nil
	},
}
