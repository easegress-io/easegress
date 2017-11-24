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

	pluginAddedCallbacks     *common.NamedCallbackSet
	pluginDeletedCallbacks   *common.NamedCallbackSet
	pluginUpdatedCallbacks   *common.NamedCallbackSet
	pipelineAddedCallbacks   *common.NamedCallbackSet
	pipelineDeletedCallbacks *common.NamedCallbackSet
	pipelineUpdatedCallbacks *common.NamedCallbackSet
}

func NewModel() *Model {
	ret := &Model{
		plugins:                  make(map[string]*Plugin),
		pluginCounter:            newPluginRefCounter(),
		pipelines:                make(map[string]*Pipeline),
		pipelineContexts:         make(map[string]pipelines.PipelineContext),
		pluginAddedCallbacks:     common.NewNamedCallbackSet(),
		pluginDeletedCallbacks:   common.NewNamedCallbackSet(),
		pluginUpdatedCallbacks:   common.NewNamedCallbackSet(),
		pipelineAddedCallbacks:   common.NewNamedCallbackSet(),
		pipelineDeletedCallbacks: common.NewNamedCallbackSet(),
		pipelineUpdatedCallbacks: common.NewNamedCallbackSet(),
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
	tmp := m.pluginAddedCallbacks.CopyCallbacks()
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
	tmp := m.pluginDeletedCallbacks.CopyCallbacks()
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

func (m *Model) ReleasePluginInstance(plugin plugins.Plugin) int64 {
	m.RLock()
	defer m.RUnlock()
	return m.pluginCounter.DeleteRef(plugin)
}

func (m *Model) DismissPluginInstanceByName(name string) error {
	m.RLock()
	defer m.RUnlock()

	plugin, exists := m.plugins[name]
	if exists {
		plugin.DismissInstance(nil)
		return nil
	} else {
		return fmt.Errorf("plugin %s not found", name)
	}
}

func (m *Model) DismissPluginInstance(instance plugins.Plugin) error {
	if instance == nil {
		return fmt.Errorf("invalid plugin instance")
	}

	m.RLock()
	defer m.RUnlock()

	plugin, exists := m.plugins[instance.Name()]
	if exists {
		plugin.DismissInstance(instance)
		return nil
	} else {
		return fmt.Errorf("plugin %s not found", instance.Name())
	}

}

func (m *Model) DismissAllPluginInstances() {
	m.RLock()
	defer m.RUnlock()

	for _, plugin := range m.plugins {
		plugin.DismissInstance(nil)
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

	tmp := m.pluginUpdatedCallbacks.CopyCallbacks()
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginUpdated)(plugin)
	}

	return nil
}

func (m *Model) AddPluginAddedCallback(name string, callback PluginAdded, priority string) {
	m.Lock()
	m.pluginAddedCallbacks = common.AddCallback(m.pluginAddedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePluginAddedCallback(name string) {
	m.Lock()
	m.pluginAddedCallbacks = common.DeleteCallback(m.pluginAddedCallbacks, name)
	m.Unlock()
}

func (m *Model) AddPluginDeletedCallback(name string, callback PluginDeleted, priority string) {
	m.Lock()
	m.pluginDeletedCallbacks = common.AddCallback(m.pluginDeletedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePluginDeletedCallback(name string) {
	m.Lock()
	m.pluginDeletedCallbacks = common.DeleteCallback(m.pluginDeletedCallbacks, name)
	m.Unlock()
}

func (m *Model) AddPluginUpdatedCallback(name string, callback PluginUpdated, priority string) {
	m.Lock()
	m.pluginUpdatedCallbacks = common.AddCallback(m.pluginUpdatedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePluginUpdatedCallback(name string) {
	m.Lock()
	m.pluginUpdatedCallbacks = common.DeleteCallback(m.pluginUpdatedCallbacks, name)
	m.Unlock()
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
	tmp := m.pipelineAddedCallbacks.CopyCallbacks()
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
	tmp := m.pipelineDeletedCallbacks.CopyCallbacks()
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

	tmp := m.pipelineUpdatedCallbacks.CopyCallbacks()
	m.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PipelineUpdated)(pipeline)
	}

	return nil
}

func (m *Model) AddPipelineAddedCallback(name string, callback PipelineAdded, priority string) {
	m.Lock()
	m.pipelineAddedCallbacks = common.AddCallback(m.pipelineAddedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePipelineAddedCallback(name string) {
	m.Lock()
	m.pipelineAddedCallbacks = common.DeleteCallback(m.pipelineAddedCallbacks, name)
	m.Unlock()
}

func (m *Model) AddPipelineDeletedCallback(name string, callback PipelineDeleted, priority string) {
	m.Lock()
	m.pipelineDeletedCallbacks = common.AddCallback(m.pipelineDeletedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePipelineDeletedCallback(name string) {
	m.Lock()
	m.pipelineDeletedCallbacks = common.DeleteCallback(m.pipelineDeletedCallbacks, name)
	m.Unlock()
}

func (m *Model) AddPipelineUpdatedCallback(name string, callback PipelineUpdated, priority string) {
	m.Lock()
	m.pipelineUpdatedCallbacks = common.AddCallback(m.pipelineUpdatedCallbacks, name, callback, priority)
	m.Unlock()
}

func (m *Model) DeletePipelineUpdatedCallback(name string) {
	m.Lock()
	m.pipelineUpdatedCallbacks = common.DeleteCallback(m.pipelineUpdatedCallbacks, name)
	m.Unlock()
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
