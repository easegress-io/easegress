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
