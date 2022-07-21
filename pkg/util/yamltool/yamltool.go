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

package yamltool

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Marshal wraps yaml.Marshal by panic instead of returning error.
func Marshal(in interface{}) []byte {
	buff, err := yaml.Marshal(in)
	if err != nil {
		panic(fmt.Errorf("marshal %s to yaml string failed: %v", in, err))
	}
	return buff
}

// Unmarshal wraps yaml.Unmarshal by panic instead of returning error.
func Unmarshal(in []byte, out interface{}) {
	err := yaml.Unmarshal(in, out)
	if err != nil {
		panic(fmt.Errorf("unmarshal yaml string %s to %#v failed: %v",
			in, out, err))
	}
}

// StructToMap converts a struct to a map based on yaml marshal.
func StructToMap(s interface{}) (map[string]interface{}, error) {
	buff, err := yaml.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal %s to yaml string failed: %v", s, err)
	}

	var m map[string]interface{}
	err = yaml.Unmarshal(buff, &m)
	if err != nil {
		return nil, fmt.Errorf("unmarshal yaml string %s to %#v failed: %v",
			buff, m, err)
	}

	return m, nil
}
