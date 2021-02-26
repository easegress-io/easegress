package supervisor

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/v"

	yaml "gopkg.in/yaml.v2"
)

type (
	// Spec is the universal spec for all objects.
	Spec struct {
		yamlConfig string
		meta       *MetaSpec
		objectSpec interface{}
	}

	// MetaSpec is metadata for all specs.
	MetaSpec struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

func newSpecInternal(meta *MetaSpec, objectSpec interface{}) *Spec {
	return &Spec{
		meta:       meta,
		objectSpec: objectSpec,
	}
}

// NewSpec creates a spec and validates it.
func NewSpec(yamlConfig string) (*Spec, error) {
	s := &Spec{
		yamlConfig: yamlConfig,
	}

	meta := &MetaSpec{}
	err := yaml.Unmarshal([]byte(yamlConfig), meta)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}
	vr := v.Validate(meta, []byte(yamlConfig))
	if !vr.Valid() {
		return nil, fmt.Errorf("validate failed: \n%s", vr)
	}

	rootObject, exists := objectRegistry[meta.Kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", meta.Kind)
	}

	s.meta, s.objectSpec = meta, rootObject.DefaultSpec()

	err = yaml.Unmarshal([]byte(yamlConfig), s.objectSpec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}
	vr = v.Validate(s.objectSpec, []byte(yamlConfig))
	if !vr.Valid() {
		return nil, fmt.Errorf("validate failed: \n%s", vr)
	}

	return s, nil
}

// Name returns name.
func (s *Spec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *Spec) Kind() string { return s.meta.Kind }

// YAMLConfig returns the config in yaml format.
func (s *Spec) YAMLConfig() string {
	return s.yamlConfig
}

// ObjectSpec returns the object spec.
func (s *Spec) ObjectSpec() interface{} {
	return s.objectSpec
}
