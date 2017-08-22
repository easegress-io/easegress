/**
 * Created by g7tianyi on 15/08/2017
 */

package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"plugins/easenight/common"
)

type MapStore struct {
	props map[string]interface{}
}

func NewMapStore() *MapStore {
	return &MapStore{props: make(map[string]interface{})}
}

func (s *MapStore) Keys() []string {
	keys := []string{}
	for key := range s.props {
		keys = append(keys, key)
	}
	return keys
}

func (s *MapStore) Put(key string, value interface{}) {
	s.props[key] = value
}

func (s *MapStore) HasValue(key string) bool {
	_, exists := s.props[key]
	return exists
}

func (s *MapStore) IntValue(key string) int {
	if val, ok := s.props[key]; ok {
		return common.ParseIntValue(val)
	}
	return 0
}

func (s *MapStore) StringValue(key string) string {
	if val, ok := s.props[key]; ok {
		return common.ParseStringValue(val)
	}
	return ""
}

func (s *MapStore) StringArrayValue(key string) []string {
	if val, ok := s.props[key]; ok {
		return common.ParseStringArray(val)
	}
	return nil
}

func (s *MapStore) ObjectValue(key string) interface{} {
	if val, ok := s.props[key]; ok {
		return val
	}
	return nil
}

func (s *MapStore) String() string {
	strArray := []string{}
	for key, val := range s.props {
		strArray = append(strArray, fmt.Sprintf("%s = %s", key, val))
	}
	return fmt.Sprintf("{ %s }", strings.Join(strArray, ","))
}

func (s *MapStore) MarshalJSON() ([]byte, error) {
	return []byte(common.Json(s)), nil
}

func (s *MapStore) UnmarshalJSON(bytes []byte) error {
	s.props = make(map[string]interface{})
	return json.Unmarshal(bytes, &s.props)
}

func (s *MapStore) MarshalYAML() (yamlStr string, err error) {
	return common.Marshal(s.props)
}

func (s *MapStore) UnmarshalYAML(yamlStr string) error {
	s.props = make(map[string]interface{})
	return common.Unmarshal(yamlStr, &s.props)
}
