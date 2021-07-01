package yamltool

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

func Marshal(in interface{}) []byte {
	buff, err := yaml.Marshal(in)
	if err != nil {
		panic(fmt.Errorf("marshal %s to yaml string failed: %v", in, err))
	}
	return buff
}

func Unmarshal(in []byte, out interface{}) {
	err := yaml.Unmarshal(in, out)
	if err != nil {
		panic(fmt.Errorf("unmarshal yaml string %s to %#v failed: %v",
			in, out, err))
	}
}
