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

package jsontool

import (
	"encoding/json"
	"fmt"
)

// TrimNull removes null values from JSON data.
// This is for backward compatibility. Null values in the original yaml configuration file was ignored by the yaml parsing functions.
func TrimNull(data []byte) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("input data can't be nil")
	}
	var f interface{}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	newObj := trimNull(f)
	return json.Marshal(newObj)
}

func trimNull(f interface{}) interface{} {
	if f == nil {
		return nil
	}
	switch v := f.(type) {
	case []interface{}:
		rst := []interface{}{}
		for _, item := range v {
			if m := trimNull(item); m != nil {
				rst = append(rst, m)
			}
		}
		return rst
	case map[string]interface{}:
		rst := make(map[string]interface{})
		for k, vv := range v {
			if m := trimNull(vv); m != nil {
				rst[k] = m
			}
		}
		return rst
	default:
		return f
	}
}
