/*
 * Copyright (c) 2017, MegaEase
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

package supervisor

import (
	"fmt"
	"reflect"

	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/megaease/easegress/pkg/v"
)

type (
	// Spec is the universal spec for all objects.
	Spec struct {
		super *Supervisor

		yamlConfig string
		meta       *MetaSpec
		rawSpec    map[string]interface{}
		objectSpec interface{}
	}

	// MetaSpec is metadata for all specs.
	MetaSpec struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

func (s *Supervisor) newSpecInternal(meta *MetaSpec, objectSpec interface{}) *Spec {
	objectBuff := yamltool.Marshal(objectSpec)
	metaBuff := yamltool.Marshal(meta)

	var rawSpec map[string]interface{}
	yamltool.Unmarshal(objectBuff, &rawSpec)
	yamltool.Unmarshal(metaBuff, &rawSpec)

	buff := yamltool.Marshal(rawSpec)
	spec, err := s.NewSpec(string(buff))
	if err != nil {
		panic(fmt.Errorf("new spec for %s failed: %v", buff, err))
	}

	return spec
}

// NewSpec is the wrapper of NewSpec of global supervisor.
func NewSpec(yamlConfig string) (*Spec, error) {
	return globalSuper.NewSpec(yamlConfig)
}

// NewSpec creates a spec and validates it.
func (s *Supervisor) NewSpec(yamlConfig string) (spec *Spec, err error) {
	spec = &Spec{super: s}

	defer func() {
		if r := recover(); r != nil {
			spec = nil
			err = fmt.Errorf("%v", r)
		} else {
			err = nil
		}
	}()

	yamlBuff := []byte(yamlConfig)

	// Meta part.
	meta := &MetaSpec{}
	yamltool.Unmarshal(yamlBuff, meta)
	verr := v.Validate(meta)
	if !verr.Valid() {
		panic(verr)
	}

	// Object self part.
	rootObject, exists := objectRegistry[meta.Kind]
	if !exists {
		panic(fmt.Errorf("kind %s not found", meta.Kind))
	}
	objectSpec := rootObject.DefaultSpec()
	yamltool.Unmarshal(yamlBuff, objectSpec)
	verr = v.Validate(objectSpec)
	if !verr.Valid() {
		panic(verr)
	}

	// Build final yaml config and raw spec.
	var rawSpec map[string]interface{}
	objectBuff := yamltool.Marshal(objectSpec)
	yamltool.Unmarshal(objectBuff, &rawSpec)

	metaBuff := yamltool.Marshal(meta)
	yamltool.Unmarshal(metaBuff, &rawSpec)

	yamlConfig = string(yamltool.Marshal(rawSpec))

	spec.meta = meta
	spec.objectSpec = objectSpec
	spec.rawSpec = rawSpec
	spec.yamlConfig = yamlConfig

	return
}

// Super returns supervisor
func (s *Spec) Super() *Supervisor {
	return s.super
}

// Name returns name.
func (s *Spec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *Spec) Kind() string { return s.meta.Kind }

// YAMLConfig returns the config in yaml format.
func (s *Spec) YAMLConfig() string {
	return s.yamlConfig
}

// RawSpec returns the final complete spec in type map[string]interface{}.
func (s *Spec) RawSpec() map[string]interface{} {
	return s.rawSpec
}

// Equals compares two Specs.
func (s *Spec) Equals(other *Spec) bool {
	return reflect.DeepEqual(s.RawSpec(), other.RawSpec())
}

// ObjectSpec returns the object spec in its own type.
func (s *Spec) ObjectSpec() interface{} {
	return s.objectSpec
}
