package registry

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/v"

	"gopkg.in/yaml.v2"
)

type (
	// Spec is all common behaviors of specs.
	Spec interface {
		GetName() string
		GetKind() string
	}

	// Object is one kind of resource, which users can create/delete/get/update/list.
	Object interface {
		Kind() string
		DefaultSpec() Spec
	}

	// object wraps metadata for every kind of object.
	object struct {
		kind        string
		defaultSpec func() Spec
	}
)

var (
	registryBook = make(map[string]Object)
)

// Register registers Object.
// It must be called in the initial stage of every object.
func Register(kind string, defaultSpec func() Spec) {
	_, exists := registryBook[kind]
	if exists {
		logger.Errorf("BUG: register failed: conflict kind %s", kind)
		return
	}
	if defaultSpec == nil {
		logger.Errorf("BUG: register failed: kind %s has nil DefaultSpec", kind)
		return
	}

	registryBook[kind] = &object{
		kind:        kind,
		defaultSpec: defaultSpec,
	}
}

func (o *object) Kind() string {
	return o.kind
}

func (o *object) DefaultSpec() Spec {
	return o.defaultSpec()
}

// Objects gets all registered objects.
func Objects() []Object {
	objs := make([]Object, 0, len(registryBook))
	for _, obj := range registryBook {
		objs = append(objs, obj)
	}

	return objs
}

func unmarshal(y string, i interface{}) error {
	err := yaml.Unmarshal([]byte(y), i)
	if err != nil {
		return fmt.Errorf("unmarshal failed: %v", err)
	}
	return v.Struct(i)
}

// SpecFromYAML validates and generates object Spec from yaml.
func SpecFromYAML(y string) (Spec, error) {
	meta := MetaSpec{}
	err := unmarshal(y, &meta)
	if err != nil {
		return nil, err
	}

	kind := meta.GetKind()
	obj, exists := registryBook[kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", kind)
	}

	spec := obj.DefaultSpec()
	err = unmarshal(y, spec)
	if err != nil {
		return nil, err
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
