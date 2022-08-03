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

package spectool

import (
	"encoding/json"
	"fmt"
	"io"

	yamljsontool "github.com/invopop/yaml"
	"gopkg.in/yaml.v3"
)

// NOTE: It's better to use functions of this package,
// since the vendor could be replaced at once if we need.

// MustMarshalJSON wraps json.Marshal by panic instead of returning error.
func MustMarshalJSON(v interface{}) []byte {
	buff, err := MarshalJSON(v)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", v, err))
	}
	return buff
}

// MustUnmarshal wraps json.MustUnmarshal by panic instead of returning error.
// It converts yaml to json before decoding by leveraging Unmarshal.
// It panics if an error occurs.
func MustUnmarshal(data []byte, v interface{}) {
	err := Unmarshal(data, v)
	if err != nil {
		panic(fmt.Errorf("unmarshal %s to %#v failed: %v",
			data, v, err))
	}
}

// MarshalJSON wraps json.MarshalJSON.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal wraps json.Unmarshal.
// It will convert yaml to json before unmarshal.
// Since json is a subset of yaml, passing json through this method should be a no-op.
func Unmarshal(data []byte, v interface{}) error {
	data, err := yamljsontool.YAMLToJSON(data)
	if err != nil {
		return fmt.Errorf("%s: convert yaml to json failed: %v", data, err)
	}
	json.Unmarshal(data, v)

	return json.Unmarshal(data, v)
}

// MustDecode decodes a json stream into a value of the given type.
// It converts yaml to json before decoding by leveraging Unmarshal.
// It panics if an error occurs.
func MustDecode(r io.Reader, v interface{}) {
	err := Decode(r, v)
	if err != nil {
		panic(err)
	}
}

// MustEncodeJSON encodes a value into a json stream.
// It panics if an error occurs.
func MustEncodeJSON(w io.Writer, v interface{}) {
	err := EncodeJSON(w, v)
	if err != nil {
		panic(err)
	}
}

// Decode decodes a json stream into a value of the given type.
// It converts yaml to json before decoding by leveraging Unmarshal.
func Decode(r io.Reader, v interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read failed: %v", err)
	}
	return Unmarshal(data, v)
}

// EncodeJSON encodes a value into a json stream.
func EncodeJSON(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// JSONToYAML converts a json stream into a yaml stream.
func JSONToYAML(in []byte) ([]byte, error) {
	return yamljsontool.JSONToYAML(in)
}

// YAMLToJSON converts a yaml stream into a json stream.
func YAMLToJSON(in []byte) ([]byte, error) {
	return yamljsontool.YAMLToJSON(in)
}

// StructToMap converts a struct to a map based on json marshal.
func StructToMap(s interface{}) (map[string]interface{}, error) {
	buff, err := MarshalJSON(s)
	if err != nil {
		return nil, fmt.Errorf("marshal %s to json string failed: %v", s, err)
	}

	var m map[string]interface{}
	err = Unmarshal(buff, &m)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json string %s to %#v failed: %v",
			buff, m, err)
	}

	return m, nil
}

// MustMarshalYAML wraps yaml.Marshal by panic instead of returning error.
func MustMarshalYAML(v interface{}) []byte {
	buff, err := MarshalYAML(v)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", v, err))
	}
	return buff
}

// yaml dedicated functions below.

// MarshalYAML wraps yaml.MarshalYAML.
func MarshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// UnmarshalYAML wraps yaml.Unmarshal.
func UnmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

// MustEncodeYAML encodes a value into a yaml stream.
// It panics if an error occurs.
func MustEncodeYAML(w io.Writer, v interface{}) {
	err := EncodeYAML(w, v)
	if err != nil {
		panic(err)
	}
}

// EncodeYAML encodes a value into a yaml stream.
func EncodeYAML(w io.Writer, v interface{}) error {
	return yaml.NewEncoder(w).Encode(v)
}
