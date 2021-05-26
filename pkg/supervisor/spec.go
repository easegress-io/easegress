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

	"github.com/megaease/easegress/pkg/v"

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
		return nil, fmt.Errorf("validate metadata failed: \n%s", vr)
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
		return nil, fmt.Errorf("validate spec failed: \n%s", vr)
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
