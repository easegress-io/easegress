package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"common"
	"config"
	"logger"
	"pipelines"
	"plugins"
)

type PluginAdded func(newPlugin *Plugin)
type PluginDeleted func(deletedPlugin *Plugin)
type PluginUpdated func(updatedPlugin *Plugin)
type PipelineAdded func(newPipeline *Pipeline)
type PipelineDeleted func(deletedPipeline *Pipeline)
type PipelineUpdated func(updatedPipeline *Pipeline)

type Model struct {
	sync.RWMutex
	plugins       map[string]*Plugin
	pluginCounter *pluginInstanceCounter
	pipelines     map[string]*Pipeline
	statistics    *statRegistry

	pluginAddedCallbacks     []*common.NamedCallback
	pluginDeletedCallbacks   []*common.NamedCallback
	pluginUpdatedCallbacks   []*common.NamedCallback
	pipelineAddedCallbacks   []*common.NamedCallback
	pipelineDeletedCallbacks []*common.NamedCallback
	pipelineUpdatedCallbacks []*common.NamedCallback
}

func NewModel() *Model {
	ret := &Model{
		plugins:       make(map[string]*Plugin),
		pluginCounter: newPluginRefCounter(),
		pipelines:     make(map[string]*Pipeline),
	}

	ret.statistics = newStatRegistry(ret)

	return ret
}

func (m *Model) LoadPlugins(specs []*config.PluginSpec) error {
	for _, spec := range specs {
		buff, err := json.Marshal(spec.Config)
		if err != nil {
			logger.Errorf("[marshal plugin config failed: %v]", err)
			return err
		}

		conf, err := plugins.GetConfig(spec.Type)
		if err != nil {
			logger.Errorf("[construct plugin config failed: %v]", err)
			return err
		}

		err = json.Unmarshal(buff, conf)
		if err != nil {
			logger.Errorf("[unmarshal plugin config failed: %v]", err)
			return err
		}

		_, err = m.AddPlugin(spec.Type, conf, spec.Constructor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Model) LoadPipelines(specs []*config.PipelineSpec) error {
	for _, spec := range specs {
		buff, err := json.Marshal(spec.Config)
		if err != nil {
			logger.Errorf("[marshal pipeline config failed: %v]", err)
			return err
		}

		conf, err := GetPipelineConfig(spec.Type)
		if err != nil {
			logger.Errorf("[construct pipeline config failed: %v]", err)
			return err
		}

		err = json.Unmarshal(buff, conf)
		if err != nil {
			logger.Errorf("[unmarshal pipeline config failed: %v]", err)
			return err
		}

		_, err = m.AddPipeline(spec.Type, conf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Model) AddPlugin(typ string, conf plugins.Config,
	constructor plugins.Constructor) (*Plugin, error) {

	m.Lock()

	pluginName := conf.PluginName()

	var pipelineNames []string
	for pipelineName := range m.pipelines {
		pipelineNames = append(pipelineNames, pipelineName)
	}

	err := conf.Prepare(pipelineNames)
	if err != nil {
		return nil, fmt.Errorf("add plugin %s failed: %v", pluginName, err)
	}

	if !plugins.ValidType(typ) {
		return nil, fmt.Errorf("plugin type %s is invalid", typ)
	}

	_, exists := m.plugins[pluginName]
	if exists {
		logger.Errorf("[add plugin %v failed: duplicate plugin]", pluginName)
		defer m.Unlock()
		return nil, fmt.Errorf("duplicate plugin %s", pluginName)
	}

	plugin := newPlugin(typ, conf, constructor, m.pluginCounter)
	m.plugins[pluginName] = plugin
	m.Unlock()

	logger.Debugf("[%d:%v registered]", len(m.plugins), pluginName)

	m.RLock()
	tmp := make([]*common.NamedCallback, len(m.pluginAddedCallbacks))
	copy(tmp, m.pluginAddedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginAdded)(plugin)
	}

	return plugin, nil
}

func (m *Model) DeletePlugin(name string) error {
	m.Lock()

	plugin, exists := m.plugins[name]
	if exists {
		for _, pipeline := range m.pipelines {
			if common.StrInSlice(name, pipeline.Config().PluginNames()) {
				m.Unlock()
				return fmt.Errorf("plugin %s is used by one or more pipelines", name)
			}
		}
	}

	delete(m.plugins, name)
	m.Unlock()
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	m.RLock()
	tmp := make([]*common.NamedCallback, len(m.pluginDeletedCallbacks))
	copy(tmp, m.pluginDeletedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginDeleted)(plugin)
	}

	return nil
}

func (m *Model) GetPlugin(name string) *Plugin {
	m.RLock()
	defer m.RUnlock()
	return m.plugins[name]
}

func (m *Model) GetPlugins(namePattern string, types []string) ([]*Plugin, error) {
	m.RLock()
	defer m.RUnlock()

	for _, t := range types {
		if !plugins.ValidType(t) {
			return nil, fmt.Errorf("invalid plugin type %s", t)
		}
	}

	if namePattern == "" {
		namePattern = `.*`
	}

	r := regexp.MustCompile(namePattern)

	var ret []*Plugin

	for _, plugin := range m.plugins {
		if len(types) > 0 && !common.StrInSlice(plugin.Type(), types) {
			continue
		}

		if r.MatchString(plugin.Name()) {
			ret = append(ret, plugin)
		}
	}

	return ret, nil
}

func (m *Model) GetPluginInstance(name string) (plugins.Plugin, error) {
	m.RLock()
	defer m.RUnlock()

	plugin, exists := m.plugins[name]
	if exists {
		instance, err := plugin.GetInstance()
		if err == nil {
			m.pluginCounter.AddRef(instance)
		}
		return instance, err
	} else {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
}

func (m *Model) ReleasePluginInstance(plugin plugins.Plugin) int {
	m.RLock()
	defer m.RUnlock()
	return m.pluginCounter.DeleteRef(plugin)
}

func (m *Model) DismissPluginInstance(name string) error {
	m.RLock()
	defer m.RUnlock()

	plugin, exists := m.plugins[name]
	if exists {
		plugin.DismissInstance()
		return nil
	} else {
		return fmt.Errorf("plugin %s not found", name)
	}
}

func (m *Model) DismissAllPluginInstances() {
	m.RLock()
	defer m.RUnlock()

	for _, plugin := range m.plugins {
		plugin.DismissInstance()
	}
}

func (m *Model) UpdatePluginConfig(conf plugins.Config) error {
	m.RLock()

	var pipelineNames []string
	for pipelineName := range m.pipelines {
		pipelineNames = append(pipelineNames, pipelineName)
	}

	err := conf.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	pluginName := conf.PluginName()

	plugin, exists := m.plugins[pluginName]
	if !exists {
		m.RUnlock()
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	plugin.UpdateConfig(conf)

	tmp := make([]*common.NamedCallback, len(m.pluginUpdatedCallbacks))
	copy(tmp, m.pluginUpdatedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginUpdated)(plugin)
	}

	return nil
}

func (m *Model) AddPluginAddedCallback(name string, callback PluginAdded, overwrite bool) PluginAdded {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginAddedCallbacks, oriCallback, _ = common.AddCallback(m.pluginAddedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginAdded)
	}
}

func (m *Model) DeletePluginAddedCallback(name string) PluginAdded {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginAddedCallbacks, oriCallback = common.DeleteCallback(m.pluginAddedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginAdded)
	}
}

func (m *Model) AddPluginDeletedCallback(name string, callback PluginDeleted, overwrite bool) PluginDeleted {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginDeletedCallbacks, oriCallback, _ = common.AddCallback(
		m.pluginDeletedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginDeleted)
	}
}

func (m *Model) DeletePluginDeletedCallback(name string) PluginDeleted {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginDeletedCallbacks, oriCallback = common.DeleteCallback(m.pluginDeletedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginDeleted)
	}
}

func (m *Model) AddPluginUpdatedCallback(name string, callback PluginUpdated, overwrite bool) PluginUpdated {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginUpdatedCallbacks, oriCallback, _ = common.AddCallback(
		m.pluginUpdatedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginUpdated)
	}
}

func (m *Model) DeletePluginUpdatedCallback(name string) PluginUpdated {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginUpdatedCallbacks, oriCallback = common.DeleteCallback(m.pluginUpdatedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginUpdated)
	}
}

func (m *Model) AddPipeline(typ string, conf pipelines.Config) (*Pipeline, error) {
	err := conf.Prepare()
	if err != nil {
		return nil, err
	}

	if !pipelines.ValidType(typ) {
		return nil, fmt.Errorf("pipeline type %s is invalid", typ)
	}

	pipelineName := conf.PipelineName()

	m.Lock()
	_, exists := m.pipelines[pipelineName]
	if exists {
		logger.Errorf("[add pipeline %v failed: duplicate pipeline]", pipelineName)
		m.Unlock()
		return nil, fmt.Errorf("pipeline %s exists", pipelineName)
	}

	for _, pluginName := range conf.PluginNames() {
		_, exists := m.plugins[pluginName]
		if !exists {
			if len(strings.TrimSpace(pluginName)) == 0 {
				pluginName = "''"
			}
			m.Unlock()
			return nil, fmt.Errorf("plugin %s not found", pluginName)
		}

		// Currently model internal allows to use plugin instance cross more than one pipeline.
		for pipelineName, pipeline := range m.pipelines {
			if common.StrInSlice(pluginName, pipeline.Config().PluginNames()) {
				m.Unlock()
				return nil, fmt.Errorf("plugin %s is used by pipeline %s", pluginName, pipelineName)
			}
		}
	}

	pipeline := newPipeline(typ, conf)
	m.pipelines[pipelineName] = pipeline
	m.Unlock()

	logger.Debugf("[%d:%v registered]", len(m.pipelines), pipelineName)

	m.RLock()
	tmp := make([]*common.NamedCallback, len(m.pipelineAddedCallbacks))
	copy(tmp, m.pipelineAddedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PipelineAdded)(pipeline)
	}

	return pipeline, nil
}

func (m *Model) DeletePipeline(name string) error {
	m.Lock()
	pipeline, exists := m.pipelines[name]
	delete(m.pipelines, name)
	m.Unlock()
	if !exists {
		return fmt.Errorf("pipeiline %s not found", name)
	}

	m.RLock()
	tmp := make([]*common.NamedCallback, len(m.pipelineDeletedCallbacks))
	copy(tmp, m.pipelineDeletedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PipelineDeleted)(pipeline)
	}

	return nil
}

func (m *Model) GetPipeline(name string) *Pipeline {
	m.RLock()
	defer m.RUnlock()
	return m.pipelines[name]
}

func (m *Model) GetPipelines(namePattern string, types []string) ([]*Pipeline, error) {
	m.RLock()
	defer m.RUnlock()

	for _, t := range types {
		if !pipelines.ValidType(t) {
			return nil, fmt.Errorf("invalid pipeline type %s", t)
		}
	}

	if namePattern == "" {
		namePattern = `.*`
	}

	r := regexp.MustCompile(namePattern)

	var ret []*Pipeline

	for _, pipeline := range m.pipelines {
		if len(types) > 0 && !common.StrInSlice(pipeline.Type(), types) {
			continue
		}

		if r.MatchString(pipeline.Name()) {
			ret = append(ret, pipeline)
		}
	}

	return ret, nil
}

func (m *Model) UpdatePipelineConfig(conf pipelines.Config) error {
	err := conf.Prepare()
	if err != nil {
		return err
	}

	pipelineName := conf.PipelineName()

	m.RLock()

	pipeline, exists := m.pipelines[pipelineName]
	if !exists {
		m.RUnlock()
		return fmt.Errorf("pipeline %s not found", pipelineName)
	}

	for _, pluginName := range conf.PluginNames() {
		_, exists := m.plugins[pluginName]
		if !exists {
			if len(strings.TrimSpace(pluginName)) == 0 {
				pluginName = "''"
			}
			m.RUnlock()
			return fmt.Errorf("plugin %s not found", pluginName)
		}
	}

	pipeline.UpdateConfig(conf)

	tmp := make([]*common.NamedCallback, len(m.pipelineUpdatedCallbacks))
	copy(tmp, m.pipelineUpdatedCallbacks)
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PipelineUpdated)(pipeline)
	}

	return nil
}

func (m *Model) AddPipelineAddedCallback(name string, callback PipelineAdded, overwrite bool) PipelineAdded {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineAddedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineAddedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineAdded)
	}
}

func (m *Model) DeletePipelineAddedCallback(name string) PipelineAdded {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineAddedCallbacks, oriCallback = common.DeleteCallback(m.pipelineAddedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineAdded)
	}
}

func (m *Model) AddPipelineDeletedCallback(name string, callback PipelineDeleted, overwrite bool) PipelineDeleted {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineDeletedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineDeletedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineDeleted)
	}
}

func (m *Model) DeletePipelineDeletedCallback(name string) PipelineDeleted {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineDeletedCallbacks, oriCallback = common.DeleteCallback(m.pipelineDeletedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineDeleted)
	}
}

func (m *Model) AddPipelineUpdatedCallback(
	name string, callback PipelineUpdated, overwrite bool) PipelineUpdated {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineUpdatedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineUpdatedCallbacks, name, callback, overwrite)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineUpdated)
	}
}

func (m *Model) DeletePipelineUpdatedCallback(name string) PipelineUpdated {
	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineUpdatedCallbacks, oriCallback = common.DeleteCallback(m.pipelineUpdatedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PipelineUpdated)
	}
}

func (m *Model) StatRegistry() *statRegistry {
	m.RLock()
	defer m.RUnlock()
	return m.statistics
}
