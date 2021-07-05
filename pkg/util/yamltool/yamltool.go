package yamltool

import (
	"fmt"

	"gopkg.in/yaml.v2"
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
