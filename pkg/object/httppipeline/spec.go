package httppipeline

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/v"
	yaml "gopkg.in/yaml.v2"
)

type (
	// FilterSpec is the universal spec for all filters.
	FilterSpec struct {
		yamlConfig string
		meta       *FilterMetaSpec
		filterSpec interface{}
		rootFilter Filter
	}

	// FilterMetaSpec is metadata for all specs.
	FilterMetaSpec struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

// NewFilterSpec creates a fileter spec.
func NewFilterSpec(meta *FilterMetaSpec, filterSpec interface{}) (*FilterSpec, error) {
	buff, err := yaml.Marshal(filterSpec)
	if err != nil {
		return nil, fmt.Errorf("marshal %# to yaml failed: %v", filterSpec, err)
	}

	var spec map[string]interface{}
	err = yaml.Unmarshal(buff, &spec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s failed: %v", buff, err)
	}

	spec["name"] = meta.Name
	spec["kind"] = meta.Kind

	return newFilterSpecInternal(spec)
}

// newFilterSpec creates a filter spec and validates it.
func newFilterSpecInternal(spec map[string]interface{}) (*FilterSpec, error) {
	yamlConfig, err := yaml.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to yaml failed: %v", spec, err)
	}

	s := &FilterSpec{
		yamlConfig: string(yamlConfig),
	}

	meta := &FilterMetaSpec{}
	err = yaml.Unmarshal(yamlConfig, meta)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	rootFilter, exists := filterRegistry[meta.Kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", rootFilter)
	}

	s.meta, s.filterSpec, s.rootFilter = meta, rootFilter.DefaultSpec(), rootFilter

	err = yaml.Unmarshal(yamlConfig, s.filterSpec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	vr := v.Validate(s.filterSpec, []byte(yamlConfig))
	if !vr.Valid() {
		return nil, fmt.Errorf("%v", vr.Error())
	}

	return s, nil
}

// Name returns name.
func (s *FilterSpec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *FilterSpec) Kind() string { return s.meta.Kind }

// YAMLConfig returns the config in yaml format.
func (s *FilterSpec) YAMLConfig() string {
	return s.yamlConfig
}

// FilterSpec returns the filter spec.
func (s *FilterSpec) FilterSpec() interface{} {
	return s.filterSpec
}

// RootFilter returns the root filter of the filter spec.
func (s *FilterSpec) RootFilter() Filter {
	return s.rootFilter
}
