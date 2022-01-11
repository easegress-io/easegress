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

package headerlookup

import (
	"fmt"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

const (
	// Kind is the kind of HeaderLookup.
	Kind = "HeaderLookup"
)

var results = []string{}

func init() {
	httppipeline.Register(&HeaderLookup{})
}

type (
	// HeaderLookup retrieves values from etcd to headers.
	HeaderLookup struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		cluster cluster.Cluster
	}

	// HeaderSetterSpec defines etcd source key and request destination header.
	HeaderSetterSpec struct {
		EtcdKey   string `yaml:"etcdKey,omitempty" jsonschema:"omitempty"`
		HeaderKey string `yaml:"headerKey,omitempty" jsonschema:"omitempty"`
	}

	// Spec defines header key and etcd prefix that form etcd key like {etcdPrefix}/{headerKey's value}.
	// This {etcdPrefix}/{headerKey's value} is retrieved from etcd and HeaderSetters extract keys from the
	// from the retrieved etcd item.
	Spec struct {
		HeaderKey     string              `yaml:"headerKey" jsonschema:"required"`
		EtcdPrefix    string              `yaml:"etcdPrefix" jsonschema:"required"`
		HeaderSetters []*HeaderSetterSpec `yaml:"headerSetters" jsonschema:"required"`
	}
)

// Validate validates spec.
func (spec Spec) Validate() error {
	if spec.HeaderKey == "" {
		return fmt.Errorf("headerKey is required")
	}
	if spec.EtcdPrefix == "" {
		return fmt.Errorf("etcdPrefix is required")
	}
	if len(spec.HeaderSetters) < 1 {
		return fmt.Errorf("at least one headerSetter is required")
	}
	for _, hs := range spec.HeaderSetters {
		if hs.EtcdKey == "" {
			return fmt.Errorf("headerSetters[i].etcdKey is required")
		}
		if hs.HeaderKey == "" {
			return fmt.Errorf("headerSetters[i].headerKey is required")
		}
	}
	return nil
}

// Kind returns the kind of HeaderLookup.
func (hl *HeaderLookup) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HeaderLookup.
func (hl *HeaderLookup) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of HeaderLookup.
func (hl *HeaderLookup) Description() string {
	return "HeaderLookup enriches request headers per request, looking up values from etcd."
}

// Results returns the results of HeaderLookup.
func (hl *HeaderLookup) Results() []string {
	return results
}

// Init initializes HeaderLookup.
func (hl *HeaderLookup) Init(filterSpec *httppipeline.FilterSpec) {
	hl.filterSpec, hl.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	if filterSpec.Super() != nil && filterSpec.Super().Cluster() != nil {
		hl.cluster = filterSpec.Super().Cluster()
	}
}

// Inherit inherits previous generation of HeaderLookup.
func (hl *HeaderLookup) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	hl.Init(filterSpec)
}

func parseYamlCreds(entry string) (map[string]string, error) {
	var err error
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("could not marshal custom-data, ensure that it's valid yaml")
		}
	}()
	yamlMap := make(map[string]string)
	yamltool.Unmarshal([]byte(entry), &yamlMap)
	return yamlMap, err
}

func (hl *HeaderLookup) lookup(headerVal string) (map[string]string, error) {
	etcdKey := hl.spec.EtcdPrefix + headerVal
	fmt.Println(etcdKey)
	etcdVal, err := hl.cluster.Get(etcdKey)
	if err != nil {
		return nil, err
	}
	if etcdVal == nil {
		return nil, fmt.Errorf("no data found")
	}
	result := make(map[string]string, len(hl.spec.HeaderSetters))
	etcdValues, err := parseYamlCreds(*etcdVal)
	if err != nil {
		return nil, err
	}
	for _, setter := range hl.spec.HeaderSetters {
		if val, ok := etcdValues[setter.EtcdKey]; ok {
			result[setter.HeaderKey] = val
		}
	}
	return result, nil
}

// Handle retrieves header values and sets request headers.
func (hl *HeaderLookup) Handle(ctx context.HTTPContext) string {
	result := hl.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (hl *HeaderLookup) handle(ctx context.HTTPContext) string {
	header := ctx.Request().Header()
	headerVal := header.Get(hl.spec.HeaderKey)
	if headerVal == "" {
		logger.Warnf("request does not have header '%s'", hl.spec.HeaderKey)
		return ""
	}
	headersToAdd, err := hl.lookup(headerVal)
	if err != nil {
		logger.Errorf(err.Error())
		return ""
	}
	for hk, hv := range headersToAdd {
		header.Set(hk, hv)
	}
	return ""
}

// Status returns status.
func (hl *HeaderLookup) Status() interface{} { return nil }

// Close closes RequestAdaptor.
func (hl *HeaderLookup) Close() {}
