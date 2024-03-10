/*
 * Copyright (c) 2017, The Easegress Authors
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
	"time"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/v"
)

// DefaultSpecVersion is the default value of the Version field in MetaSpec.
const DefaultSpecVersion = "easegress.megaease.com/v2"

type (
	// Spec is the universal spec for all objects.
	Spec struct {
		super *Supervisor

		category   ObjectCategory
		jsonConfig string
		meta       *MetaSpec
		rawSpec    map[string]interface{}
		objectSpec interface{}
	}

	// MetaSpec is metadata for all specs.
	MetaSpec struct {
		Name    string `json:"name" jsonschema:"required,format=urlname"`
		Kind    string `json:"kind" jsonschema:"required"`
		Version string `json:"version,omitempty"`

		// RFC3339 format
		CreatedAt string `json:"createdAt,omitempty"`
	}
)

func (s *Supervisor) newSpecInternal(meta *MetaSpec, objectSpec interface{}) *Spec {
	objectBuff := codectool.MustMarshalJSON(objectSpec)
	metaBuff := codectool.MustMarshalJSON(meta)

	var rawSpec map[string]interface{}
	codectool.MustUnmarshal(objectBuff, &rawSpec)
	codectool.MustUnmarshal(metaBuff, &rawSpec)

	buff := codectool.MustMarshalJSON(rawSpec)
	spec, err := s.NewSpec(string(buff))
	if err != nil {
		panic(fmt.Errorf("new spec for %s failed: %v", buff, err))
	}

	return spec
}

// NewSpec is the wrapper of NewSpec of global supervisor.
func NewSpec(config string) (*Spec, error) {
	return globalSuper.NewSpec(config)
}

func NewMeta(kind, name string) *MetaSpec {
	return &MetaSpec{
		Name:      name,
		Kind:      kind,
		Version:   DefaultSpecVersion,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

// NewSpec creates a spec and validates it from the config in json format.
// Config supports both json and yaml format.
func (s *Supervisor) NewSpec(config string) (spec *Spec, err error) {
	return s.newSpec(config, false)
}

// CreateSpec is like NewSpec, but it sets the CreatedAt field in MetaSpec.
// It should be used to create or update object spec.
func (s *Supervisor) CreateSpec(config string) (spec *Spec, err error) {
	return s.newSpec(config, true)
}

func (s *Supervisor) newSpec(config string, created bool) (spec *Spec, err error) {
	spec = &Spec{super: s}

	defer func() {
		if r := recover(); r != nil {
			spec = nil
			err = fmt.Errorf("%v", r)
		} else {
			err = nil
		}
	}()

	buff := []byte(config)

	// Meta part.
	meta := &MetaSpec{Version: DefaultSpecVersion}
	if created {
		meta.CreatedAt = time.Now().Format(time.RFC3339)
	}
	codectool.MustUnmarshal(buff, meta)
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
	codectool.MustUnmarshal(buff, objectSpec)
	verr = v.Validate(objectSpec)
	if !verr.Valid() {
		panic(verr)
	}

	// Build final json config and raw spec.
	var rawSpec map[string]interface{}
	objectBuff := codectool.MustMarshalJSON(objectSpec)
	codectool.MustUnmarshal(objectBuff, &rawSpec)

	metaBuff := codectool.MustMarshalJSON(meta)
	codectool.MustUnmarshal(metaBuff, &rawSpec)

	jsonConfig := string(codectool.MustMarshalJSON(rawSpec))

	spec.category = rootObject.Category()
	spec.meta = meta
	spec.objectSpec = objectSpec
	spec.rawSpec = rawSpec
	spec.jsonConfig = jsonConfig

	return
}

// Super returns supervisor
func (s *Spec) Super() *Supervisor {
	return s.super
}

// MarshalJSON marshals the spec to json.
func (s *Spec) MarshalJSON() ([]byte, error) {
	return []byte(s.jsonConfig), nil
}

func (s *Spec) Categroy() ObjectCategory {
	return s.category
}

// Name returns name.
func (s *Spec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *Spec) Kind() string { return s.meta.Kind }

// Version returns version.
func (s *Spec) Version() string { return s.meta.Version }

// JSONConfig returns the config in json format.
func (s *Spec) JSONConfig() string {
	return s.jsonConfig
}

// RawSpec returns the final complete spec in type map[string]interface{}.
func (s *Spec) RawSpec() map[string]interface{} {
	return s.rawSpec
}

// ObjectSpec returns the object spec in its own type.
func (s *Spec) ObjectSpec() interface{} {
	return s.objectSpec
}

// Equals compares two Specs.
func (s *Spec) Equals(other *Spec) bool {
	return reflect.DeepEqual(s.RawSpec(), other.RawSpec())
}
