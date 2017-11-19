package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"

	"common"
	"config"
	"logger"
	pipelines_gw "pipelines"
	plugins_gw "plugins"
)

// safe characters for friendly url, rfc3986 section 2.3
var PIPELINE_PLUGIN_NAME_REGEX = regexp.MustCompile(`^[A-Za-z0-9\-_\.~]+$`)

type PluginAdded func(newPlugin *Plugin)
type PluginDeleted func(deletedPlugin *Plugin)
type PluginUpdated func(updatedPlugin *Plugin)
type PipelineAdded func(newPipeline *Pipeline)
type PipelineDeleted func(deletedPipeline *Pipeline)
type PipelineUpdated func(updatedPipeline *Pipeline)

type Model struct {
	sync.RWMutex
	plugins          map[string]*Plugin
	pluginCounter    *pluginInstanceCounter
	pipelines        map[string]*Pipeline
	pipelineContexts map[string]pipelines.PipelineContext
	statistics       *statRegistry

	pluginAddedCallbacks     []*common.NamedCallback
	pluginDeletedCallbacks   []*common.NamedCallback
	pluginUpdatedCallbacks   []*common.NamedCallback
	pipelineAddedCallbacks   []*common.NamedCallback
	pipelineDeletedCallbacks []*common.NamedCallback
	pipelineUpdatedCallbacks []*common.NamedCallback
}

func NewModel() *Model {
	ret := &Model{
		plugins:                  make(map[string]*Plugin),
		pluginCounter:            newPluginRefCounter(),
		pipelines:                make(map[string]*Pipeline),
		pipelineContexts:         make(map[string]pipelines.PipelineContext),
		pluginAddedCallbacks:     make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
		pluginDeletedCallbacks:   make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
		pluginUpdatedCallbacks:   make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
		pipelineAddedCallbacks:   make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
		pipelineDeletedCallbacks: make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
		pipelineUpdatedCallbacks: make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
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

		conf, err := plugins_gw.GetConfig(spec.Type)
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

	pluginName := conf.PluginName()

	if !PIPELINE_PLUGIN_NAME_REGEX.Match([]byte(pluginName)) {
		return nil, fmt.Errorf("plugin name %s is invalid", pluginName)
	}

	if !plugins_gw.ValidType(typ) {
		return nil, fmt.Errorf("plugin type %s is invalid", typ)
	}

	m.Lock()

	_, exists := m.plugins[pluginName]
	if exists {
		logger.Errorf("[add plugin %v failed: duplicated plugin]", pluginName)
		m.Unlock()
		return nil, fmt.Errorf("duplicated plugin %s", pluginName)
	}

	var pipelineNames []string
	for pipelineName := range m.pipelines {
		pipelineNames = append(pipelineNames, pipelineName)
	}

	err := conf.Prepare(pipelineNames)
	if err != nil {
		m.Unlock()
		return nil, fmt.Errorf("prepare plugin %s failed: %v", pluginName, err)
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

func (m *Model) GetPlugin(name string) (*Plugin, int) {
	m.RLock()
	defer m.RUnlock()

	plugin := m.plugins[name]
	if plugin == nil {
		return plugin, 0
	}

	var refCount int
	for _, pipeline := range m.pipelines {
		if common.StrInSlice(name, pipeline.Config().PluginNames()) {
			refCount++
		}
	}

	return plugin, refCount
}

func (m *Model) GetPlugins(namePattern string, types []string) ([]*Plugin, error) {
	m.RLock()
	defer m.RUnlock()

	for _, t := range types {
		if !plugins_gw.ValidType(t) {
			return nil, fmt.Errorf("invalid plugin type %s", t)
		}
	}

	if len(namePattern) == 0 {
		namePattern = `.*`
	}

	var ret []*Plugin

	r, err := regexp.Compile(namePattern)
	if err != nil {
		return ret, fmt.Errorf("invalid plugin name pattern: %v", err)
	}

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
		instance, err := plugin.GetInstance(m)
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

	pluginName := conf.PluginName()

	plugin, exists := m.plugins[pluginName]
	if !exists {
		m.RUnlock()
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	var pipelineNames []string
	for pipelineName := range m.pipelines {
		pipelineNames = append(pipelineNames, pipelineName)
	}

	err := conf.Prepare(pipelineNames)
	if err != nil {
		m.RUnlock()
		return fmt.Errorf("prepare plugin %s failed: %v", pluginName, err)
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

func (m *Model) AddPluginAddedCallback(name string, callback PluginAdded,
	overwrite bool, priority string) PluginAdded {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginAddedCallbacks, oriCallback, _ = common.AddCallback(
		m.pluginAddedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) AddPluginDeletedCallback(name string, callback PluginDeleted,
	overwrite bool, priority string) PluginDeleted {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginDeletedCallbacks, oriCallback, _ = common.AddCallback(
		m.pluginDeletedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) AddPluginUpdatedCallback(name string, callback PluginUpdated,
	overwrite bool, priority string) PluginUpdated {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pluginUpdatedCallbacks, oriCallback, _ = common.AddCallback(
		m.pluginUpdatedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) AddPipeline(typ string, conf pipelines_gw.Config) (*Pipeline, error) {
	pipelineName := conf.PipelineName()

	if !PIPELINE_PLUGIN_NAME_REGEX.Match([]byte(pipelineName)) {
		return nil, fmt.Errorf("pipeline name %s is invalid", pipelineName)
	}

	if !pipelines_gw.ValidType(typ) {
		return nil, fmt.Errorf("pipeline type %s is invalid", typ)
	}

	m.Lock()

	_, exists := m.pipelines[pipelineName]
	if exists {
		logger.Errorf("[add pipeline %v failed: duplicated pipeline]", pipelineName)
		m.Unlock()
		return nil, fmt.Errorf("duplicated pipeline %s", pipelineName)
	}

	err := conf.Prepare()
	if err != nil {
		m.Unlock()
		return nil, fmt.Errorf("prepare pipeline %s failed: %v", pipelineName, err)
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
		if !pipelines_gw.ValidType(t) {
			return nil, fmt.Errorf("invalid pipeline type %s", t)
		}
	}

	if len(namePattern) == 0 {
		namePattern = `.*`
	}

	var ret []*Pipeline

	r, err := regexp.Compile(namePattern)
	if err != nil {
		return ret, fmt.Errorf("invalid plugin name pattern: %v", err)
	}

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

func (m *Model) UpdatePipelineConfig(conf pipelines_gw.Config) error {
	pipelineName := conf.PipelineName()

	err := conf.Prepare()
	if err != nil {
		return fmt.Errorf("prepare pipeline %s failed: %v", pipelineName, err)
	}

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

func (m *Model) AddPipelineAddedCallback(name string, callback PipelineAdded,
	overwrite bool, priority string) PipelineAdded {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineAddedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineAddedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) AddPipelineDeletedCallback(name string, callback PipelineDeleted,
	overwrite bool, priority string) PipelineDeleted {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineDeletedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineDeletedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) AddPipelineUpdatedCallback(name string, callback PipelineUpdated,
	overwrite bool, priority string) PipelineUpdated {

	m.Lock()
	defer m.Unlock()

	var oriCallback interface{}
	m.pipelineUpdatedCallbacks, oriCallback, _ = common.AddCallback(
		m.pipelineUpdatedCallbacks, name, callback, overwrite, priority)

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

func (m *Model) CreatePipelineContext(
	conf pipelines_gw.Config, statistics pipelines.PipelineStatistics) pipelines.PipelineContext {

	ctx := NewPipelineContext(conf, statistics, m)

	m.Lock()
	defer m.Unlock()

	m.pipelineContexts[conf.PipelineName()] = ctx

	return ctx
}

func (m *Model) DeletePipelineContext(name string) bool {
	m.Lock()
	defer m.Unlock()

	ctx, exists := m.pipelineContexts[name]
	if exists {
		ctx.Close()

		delete(m.pipelineContexts, name)
	}

	return exists
}

func (m *Model) GetPipelineContext(name string) pipelines.PipelineContext {
	m.RLock()
	defer m.RUnlock()
	return m.pipelineContexts[name]
}

func (m *Model) StatRegistry() *statRegistry {
	m.RLock()
	defer m.RUnlock()
	return m.statistics
}
