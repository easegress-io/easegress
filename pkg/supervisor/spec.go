package supervisor

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/v"

	yaml "gopkg.in/yaml.v2"
)

type (
	// ObjectSpec is the common interface for all object spec.
	ObjectSpec interface {
		GetName() string
		GetKind() string
	}

	// ObjectMetaSpec is the basic spec for all objects.
	ObjectMetaSpec struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

// GetName returns name.
func (s *ObjectMetaSpec) GetName() string { return s.Name }

// GetKind returns kind.
func (s *ObjectMetaSpec) GetKind() string { return s.Kind }

func unmarshal(y string, i interface{}) error {
	err := yaml.Unmarshal([]byte(y), i)
	if err != nil {
		return fmt.Errorf("unmarshal failed: %v", err)
	}

	yamlBuff, err := yaml.Marshal(i)
	if err != nil {
		return fmt.Errorf("marshal %#v failed: %v", i, err)
	}

	vr := v.Validate(i, yamlBuff)
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr)
	}

	return nil
}

// SpecFromYAML validates and generates object Spec from yaml.
func SpecFromYAML(y string) (ObjectSpec, error) {
	meta := ObjectMetaSpec{}
	err := unmarshal(y, &meta)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	kind := meta.GetKind()
	o, exists := objectRegistry[kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", kind)
	}

	spec := o.DefaultSpec()

	err = unmarshal(y, spec)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	return spec, nil
}

// YAMLFromSpec is an utility to transfer spec to yaml.
func YAMLFromSpec(spec ObjectSpec) string {
	y, err := yaml.Marshal(spec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", err)
		return ""
	}
	return string(y)
}
