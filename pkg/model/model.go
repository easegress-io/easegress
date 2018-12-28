package model

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/plugins"
	"github.com/megaease/easegateway/pkg/store"
)

type pluginUpdateInfo struct {
	instanceDismissed  bool
	instanceGeneration uint64
	plugin             *Plugin
}

type Model struct {
	pluginsLock    sync.RWMutex
	plugins        map[string]*Plugin
	pluginsCounter *pluginInstanceCounter

	schedulersLock sync.RWMutex
	schedulers     map[string]PipelineScheduler

	store   store.Store
	watcher *store.Watcher
}

func NewModel(store store.Store) (*Model, error) {
	watcher, err := store.ClaimWatcher("model")
	if err != nil {
		return nil, fmt.Errorf("claim watcher failed: %v", err)
	}
	m := &Model{
		plugins:        make(map[string]*Plugin),
		pluginsCounter: newPluginRefCounter(),
		schedulers:     make(map[string]PipelineScheduler),
		store:          store,
		watcher:        watcher,
	}
	go func() {
		for diffSpec := range m.watcher.Watch() {
			m.applyDiffSpec(diffSpec)
		}
	}()
	return m, nil
}

func (m *Model) preparePluginInstance(plugin plugins.Plugin) {
	for _, ctx := range m.pipelineContexts() {
		if common.StrInSlice(plugin.Name(), ctx.PluginNames()) {
			plugin.Prepare(ctx)
		}
	}
}

func (m *Model) getPluginInstance(name string, prepareForNew bool) (*wrappedPlugin, plugins.PluginType, uint64, error) {
	m.pluginsLock.RLock()
	plugin, exists := m.plugins[name]
	if !exists {
		return nil, plugins.UnknownType, 0, fmt.Errorf("plugin %s not found", name)
	}
	m.pluginsLock.RUnlock()

	instance, pluginType, gen, err := plugin.GetInstance(m, prepareForNew)
	if err == nil {
		m.pluginsCounter.AddRef(instance)
	}

	return instance, pluginType, gen, err
}

func (m *Model) releasePluginInstance(plugin *wrappedPlugin) int64 {
	return m.pluginsCounter.DeleteRef(plugin)
}

func (m *Model) dismissPluginInstance(instance *wrappedPlugin) error {
	if instance == nil {
		return fmt.Errorf("invalid plugin instance")
	}

	m.pluginsLock.RLock()
	defer m.pluginsLock.RUnlock()

	plugin, exists := m.plugins[instance.Name()]
	if !exists {
		return fmt.Errorf("plugin %s not found", instance.Name())
	}

	plugin.DismissInstance(instance)

	return nil
}

func (m *Model) dismissPluginInstanceByName(name string) error {
	m.pluginsLock.RLock()
	defer m.pluginsLock.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	plugin.DismissInstance(nil)

	return nil
}

func (m *Model) dismissAllPluginInstances() {
	m.pluginsLock.RLock()
	defer m.pluginsLock.RUnlock()

	for _, plugin := range m.plugins {
		plugin.dismissInstance(nil)
	}
}

func (m *Model) createPlugin(spec *store.PluginSpec) {
	m.pluginsLock.Lock()
	defer m.pluginsLock.Unlock()

	if _, exists := m.plugins[spec.Name]; exists {
		logger.Errorf("[BUG: plugin %s existed]", spec.Name)
		return
	}

	m.plugins[spec.Name] = newPlugin(spec, m.pluginsCounter)
}

func (m *Model) deletePlugin(name string) {
	m.pluginsLock.Lock()
	defer m.pluginsLock.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		logger.Errorf("[BUG: plugin %s not found]", name)
		return
	}
	delete(m.plugins, name)

	go func() {
		for _, ctx := range m.pipelineContexts() {
			ctx.pluginDeleteChan <- plugin
		}
	}()
}

func (m *Model) updatePlugin(spec *store.PluginSpec) {
	m.pluginsLock.Lock()
	defer m.pluginsLock.Unlock()

	plugin, exists := m.plugins[spec.Name]
	if !exists {
		logger.Errorf("[BUG: plugin %s not found]", spec.Name)
		return
	}

	dismissed, generation := plugin.UpdateConfig(spec.Config.(plugins.Config))
	go func() {
		for _, instances := range m.pipelineInstances() {
			for _, instance := range instances {
				instance.instance.pluginUpdateInfoChan <- &pluginUpdateInfo{
					instanceDismissed:  dismissed,
					instanceGeneration: generation,
					plugin:             plugin,
				}
			}
		}
	}()
}

func (m *Model) createPipeline(spec *store.PipelineSpec) {
	m.schedulersLock.Lock()
	defer m.schedulersLock.Unlock()

	if _, exists := m.schedulers[spec.Name]; exists {
		logger.Errorf("[BUG: pipeline scheduler %s existed]", spec.Name)
		return
	}
	scheduler := CreatePipelineScheduler(spec, m)
	m.schedulers[spec.Name] = scheduler

	scheduler.Start()
}

func (m *Model) deletePipeline(name string) {
	m.schedulersLock.Lock()
	defer m.schedulersLock.Unlock()

	scheduler, exists := m.schedulers[name]
	if !exists {
		logger.Errorf("[BUG: pipeline scheduler %s not found]", name)
		return
	}
	scheduler.Stop()
	delete(m.schedulers, name)
}

func (m *Model) updatePipeline(spec *store.PipelineSpec) {
	m.schedulersLock.Lock()
	defer m.schedulersLock.Unlock()

	scheduler, exists := m.schedulers[spec.Name]
	if !exists {
		logger.Errorf("[BUG: pipeline scheduler %s not found]", spec.Name)
		return
	}
	scheduler.Stop()

	scheduler = CreatePipelineScheduler(spec, m)
	m.schedulers[spec.Name] = CreatePipelineScheduler(spec, m)
	scheduler.Start()
}

func (m *Model) pipelineContexts() map[string]*pipelineContext {
	m.schedulersLock.RLock()
	defer m.schedulersLock.RUnlock()

	contexts := make(map[string]*pipelineContext)
	for name, scheduler := range m.schedulers {
		contexts[name] = scheduler.PipelineContext()
	}

	return contexts
}

func (m *Model) pipelineInstances() map[string][]*pipelineInstance {
	m.schedulersLock.RLock()
	defer m.schedulersLock.RUnlock()

	pipelineInstances := make(map[string][]*pipelineInstance)
	for name, scheduler := range m.schedulers {
		pipelineInstances[name] = scheduler.PipelineInstances()
	}

	return pipelineInstances
}

func (m *Model) applyDiffSpec(diffSpec *store.DiffSpec) {
	for _, pluginSpec := range diffSpec.CreatedOrUpdatedPlugins {
		m.createPlugin(pluginSpec)
	}
	for _, pipelineSpec := range diffSpec.CreatedOrUpdatedPipelines {
		m.createPipeline(pipelineSpec)
	}

	for _, name := range diffSpec.DeletedPipelines {
		m.deletePipeline(name)
	}

	for _, name := range diffSpec.DeletedPlugins {
		m.deletePlugin(name)
	}
}

func (m *Model) Close() {
	m.store.DeleteWatcher("model")
	for name := range m.schedulers {
		m.deletePipeline(name)
	}
	for name := range m.plugins {
		m.deletePlugin(name)
	}
}
