package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/plugins"
)

// Store is the middle level between cluster and model for config.
type Store interface {
	// Watch channel always sends the whole Spec at first time,
	// then sends each change afterwards.
	Watch() <-chan *Event

	Close()
}

func New(cluster cluster.Cluster) (Store, error) {
	store, err := newJSONFileStore(cluster)
	if err != nil {
		return nil, err
	}

	return store, nil
}

type (
	PluginSpec struct {
		Type        string              `json:"type"`
		Name        string              `json:"name"`
		Config      interface{}         `json:"config"`
		Constructor plugins.Constructor `json:"-"`
	}

	PipelineSpec struct {
		Type   string                          `json:"type"`
		Name   string                          `json:"name"`
		Config *pipelines.PipelineCommonConfig `json:"config"`
	}

	Event struct {
		Pipelines map[string]*PipelineSpec
		Plugins   map[string]*PluginSpec
	}
)

func NewPluginSpec(value string) (*PluginSpec, error) {
	spec := new(PluginSpec)
	err := json.Unmarshal([]byte(value), spec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s to json failed: %v",
			value, err)
	}
	return spec, nil
}

func (spec *PluginSpec) Bootstrap(pipelineNames []string) error {
	constructor, config, err := plugins.GetConstructorConfig(spec.Type)
	if err != nil {
		return fmt.Errorf("get contructor and config for plugin %s(type: %s) failed: %v",
			spec.Name, spec.Type, err)
	}

	buff, err := json.Marshal(spec.Config)
	if err != nil {
		return fmt.Errorf("marshal %#v failed: %v", spec.Config, err)
	}
	err = json.Unmarshal(buff, config)
	if err != nil {
		return fmt.Errorf("unmarshal %s for config of plugin %s(type %s) failed: %v",
			buff, spec.Name, spec.Type, err)
	}
	err = config.Prepare(pipelineNames)
	if err != nil {
		return fmt.Errorf("prepare config for plugin %s(type %s) failed: %v",
			spec.Name, spec.Type, err)
	}

	spec.Constructor, spec.Config = constructor, config

	return nil
}

// NOTICE: The field Config of PluginSepc could be map[string]interface{}
// in general after unmarshal, but it would be converted the plugins.Config
// which contains the real plugin config structure. So it has to get equality
// by checking literal []byte.
func (spec *PluginSpec) equal(other *PluginSpec) bool {
	buff1, err := json.Marshal(spec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", spec, err)
		return false
	}
	buff2, err := json.Marshal(other)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", other, err)
		return false
	}

	return bytes.Equal(buff1, buff2)
}

func NewPipelineSpec(value string) (*PipelineSpec, error) {
	spec := new(PipelineSpec)
	err := json.Unmarshal([]byte(value), spec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s to json failed: %v",
			value, err)
	}
	return spec, nil
}

func (spec *PipelineSpec) Bootstrap(pluginNames map[string]struct{}) error {
	err := spec.Config.Prepare()
	if err != nil {
		return fmt.Errorf("prepare config for pipeline %s failed: %v",
			spec.Name, err)
	}

	for _, pluginName := range spec.Config.Plugins {
		if _, exists := pluginNames[pluginName]; !exists {
			return fmt.Errorf("plugin %s not found", pluginName)
		}
	}

	return nil
}

func (spec *PipelineSpec) equal(other *PipelineSpec) bool {
	return reflect.DeepEqual(spec, other)
}
