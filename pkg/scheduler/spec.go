package scheduler

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/v"

	yaml "gopkg.in/yaml.v2"
)

type (
	// Spec is the common interface for all object spec.
	Spec interface {
		GetName() string
		GetKind() string
	}

	// ObjectMeta is the fundamental specification for all objects
	// which want to be scheduled.
	ObjectMeta struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

// GetName returns name.
func (om *ObjectMeta) GetName() string { return om.Name }

// GetKind returns kind.
func (om *ObjectMeta) GetKind() string { return om.Kind }

func unmarshal(y string, i interface{}) error {
	err := yaml.Unmarshal([]byte(y), i)
	if err != nil {
		return fmt.Errorf("unmarshal failed: %v", err)
	}

	vr := v.Validate(i, []byte(y))
	if !vr.Valid() {
		return fmt.Errorf("validate failed: \n%s", vr)
	}

	return nil
}

// SpecFromYAML validates and generates object Spec from yaml.
func SpecFromYAML(y string) (Spec, error) {
	meta := ObjectMeta{}
	err := unmarshal(y, &meta)
	if err != nil {
		return nil, err
	}

	kind := meta.GetKind()
	or, exists := objectBook[kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", kind)
	}

	defaultSpec := reflect.ValueOf(or.DefaultSpecFunc).Call(nil)[0].Interface()

	err = unmarshal(y, defaultSpec)
	if err != nil {
		return nil, err
	}

	spec, ok := defaultSpec.(Spec)
	if !ok {
		return nil, fmt.Errorf("invalid spec: not implement scheduler.Spec")
	}

	return spec, nil
}

// YAMLFromSpec is an utility to transfer spec to yaml.
func YAMLFromSpec(spec Spec) string {
	y, err := yaml.Marshal(spec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", err)
		return ""
	}
	return string(y)
}
