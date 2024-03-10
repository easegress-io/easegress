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

// Package codectool provides some codec tools for JSON and YAML marshaling.
package codectool

import (
	"encoding/json"
	"fmt"
	"io"

	yamljsontool "github.com/invopop/yaml"
	"github.com/megaease/yaml"
)

// NOTE: It's better to use functions of this package,
// since the vendor could be replaced at once if we need.

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

// Unmarshal wraps json.Unmarshal.
// It will convert yaml to json before unmarshal.
// Since json is a subset of yaml, passing json through this method should be a no-op.
func Unmarshal(data []byte, v interface{}) error {
	data, err := yamljsontool.YAMLToJSON(data)
	if err != nil {
		return fmt.Errorf("%s: convert yaml to json failed: %v", data, err)
	}

	return json.Unmarshal(data, v)
}

// MustMarshalJSON wraps json.Marshal by panic instead of returning error.
func MustMarshalJSON(v interface{}) []byte {
	buff, err := MarshalJSON(v)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", v, err))
	}
	return buff
}

// MarshalJSON wraps json.Marshal.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON wraps json.Unmarshal.
func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// MustUnmarshalJSON wraps json.Unmarshal.
// It panics if an error occurs.
func MustUnmarshalJSON(data []byte, v interface{}) {
	err := Unmarshal(data, v)
	if err != nil {
		panic(fmt.Errorf("unmarshal json %s to %#v failed: %v",
			data, v, err))
	}
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

// Decode decodes a json stream into a value of the given type.
// It converts yaml to json before decoding by leveraging Unmarshal.
func Decode(r io.Reader, v interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read failed: %v", err)
	}
	return Unmarshal(data, v)
}

// MustDecodeJSON decodes a json stream into a value of the given type.
// It panics if an error occurs.
func MustDecodeJSON(r io.Reader, v interface{}) {
	err := DecodeJSON(r, v)
	if err != nil {
		panic(err)
	}
}

// DecodeJSON decodes a json stream into a value of the given type.
func DecodeJSON(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// MustEncodeJSON encodes a value into a json stream.
// It panics if an error occurs.
func MustEncodeJSON(w io.Writer, v interface{}) {
	err := EncodeJSON(w, v)
	if err != nil {
		panic(err)
	}
}

// EncodeJSON encodes a value into a json stream.
func EncodeJSON(w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

// MustJSONToYAML converts a json stream into a yaml stream.
// It panics if an error occurs.
func MustJSONToYAML(in []byte) []byte {
	buff, err := JSONToYAML(in)
	if err != nil {
		panic(fmt.Errorf("json %s to yaml failed: %v", in, err))
	}
	return buff
}

// JSONToYAML converts a json stream into a yaml stream.
func JSONToYAML(in []byte) ([]byte, error) {
	return yamljsontool.JSONToYAML(in)
}

// MustYAMLToJSON converts a json stream into a yaml stream.
// It panics if an error occurs.
func MustYAMLToJSON(in []byte) []byte {
	buff, err := YAMLToJSON(in)
	if err != nil {
		panic(fmt.Errorf("yaml %s to json failed: %v", in, err))
	}
	return buff
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

// yaml dedicated functions below.
// NOTICE: We use megaease-forked yaml vendor to encode/decode yaml,
// it will use json tag if yaml tag not found.

// MustMarshalYAML wraps yaml.Marshal by panic instead of returning error.
func MustMarshalYAML(v interface{}) []byte {
	buff, err := MarshalYAML(v)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", v, err))
	}
	return buff
}

// MarshalYAML wraps yaml.MarshalYAML.
func MarshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// MustUnmarshalYAML wraps yaml.Unmarshal by panic instead of returning error.
func MustUnmarshalYAML(data []byte, v interface{}) {
	err := UnmarshalYAML(data, v)
	if err != nil {
		panic(fmt.Errorf("unmarshal yaml %s to %#v failed: %v",
			data, v, err))
	}
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

// DecodeYAML decodes a yaml stream into a value of the given type.
func DecodeYAML(r io.Reader, v interface{}) error {
	return yaml.NewDecoder(r).Decode(v)
}

// MustDecodeYAML decodes a yaml stream into a value of the given type.
// It panics if an error occurs.
func MustDecodeYAML(r io.Reader, v interface{}) {
	err := DecodeYAML(r, v)
	if err != nil {
		panic(err)
	}
}
