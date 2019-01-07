package store

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/plugins"
)

type Store interface {
	GetPlugin(name string) *PluginSpec
	GetPipeline(name string) *PipelineSpec
	ListPlugins() map[string]*PluginSpec
	ListPipelines() map[string]*PipelineSpec
	ListPluginPipelines() (map[string]*PluginSpec, map[string]*PipelineSpec)
	CreatePlugin(pluginSpec *PluginSpec) error
	CreatePipeline(pipelineSpec *PipelineSpec) error
	DeletePlugin(name string) error
	DeletePipeline(name string) error
	UpdatePlugin(pluginSpec *PluginSpec) error
	UpdatePipeline(pipelineSpec *PipelineSpec) error
	ApplyDiff(diffSpec *DiffSpec) error
	// AddWatcher adds a watcher of store,
	// the store always sends the whole Spec at first time,
	// then sends each change afterwards.
	ClaimWatcher(name string) (*Watcher, error)
	DeleteWatcher(name string)
	Close()
}

func New() (Store, error) {
	store, err := newJSONFileStore()
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

	DiffSpec struct {
		Total                     bool
		CreatedOrUpdatedPipelines map[string]*PipelineSpec
		DeletedPipelines          []string
		CreatedOrUpdatedPlugins   map[string]*PluginSpec
		DeletedPlugins            []string
	}
	Watcher struct {
		diffSpecChan chan *DiffSpec
	}
)

// NOTICE: The field Config of PluginSepc could be map[string]interface{}
// in general after unmarshal, but it would be converted the plugins.Config
// which contains the real plugin config structure. So it has to get equality
// by checking literal []byte.
func (spec *PluginSpec) equal(other *PluginSpec) bool {
	buff1, err := json.Marshal(spec)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v to json failed: %v]", spec, err)
		return false
	}
	buff2, err := json.Marshal(other)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v to json failed: %v]", other, err)
		return false
	}

	return bytes.Equal(buff1, buff2)
}

func (spec *PipelineSpec) equal(other *PipelineSpec) bool {
	return reflect.DeepEqual(spec, other)
}

func newWatcher() *Watcher {
	return &Watcher{
		diffSpecChan: make(chan *DiffSpec, 10),
	}
}

func (w *Watcher) Watch() <-chan *DiffSpec {
	return w.diffSpecChan
}

func (w *Watcher) close() {
	close(w.diffSpecChan)
}
