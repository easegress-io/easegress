/**
 * Created by g7tianyi on 26/05/2017
 */

package common

import (
	"github.com/ghodss/yaml"
)

type Marshaler interface {
	MarshalYAML() (yamlStr string, err error)
}

type Unmarshaler interface {
	UnmarshalYAML(yamlStr string) error
}

// Marshal an object into YAML string
func Marshal(obj interface{}) (str string, err error) {
	var yamlBytes []byte
	yamlBytes, err = yaml.Marshal(obj)
	if err != nil {
		return
	}
	str = string(yamlBytes)
	return
}

// Unmarshal a YAML string into the target object.
// Please note the `obj` must be a pointer
// e.g., yaml.Deserialize(yourYAMLString, &myObject)
func Unmarshal(yamlStr string, obj interface{}) error {
	return yaml.Unmarshal([]byte(yamlStr), obj)
}
