package model

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
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

	store store.Store
}

func NewModel(store store.Store) (*Model, error) {
	m := &Model{
		plugins:        make(map[string]*Plugin),
		pluginsCounter: newPluginRefCounter(),
		schedulers:     make(map[string]PipelineScheduler),
		store:          store,
	}
	go func() {
		for event := range m.store.Watch() {
			m.handlEvent(event)
		}
	}()
	return m, nil
}

func (m *Model) PipelineStats() map[string]pipelines.PipelineStatistics {
	m.schedulersLock.RLock()
	defer m.schedulersLock.RUnlock()

	stats := make(map[string]pipelines.PipelineStatistics)
	for name, scheduler := range m.schedulers {
		stats[name] = scheduler.PipelineContext().Statistics()
	}

	return stats
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
		logger.Errorf("BUG: plugin %s existed", spec.Name)
		return
	}

	m.plugins[spec.Name] = newPlugin(spec, m.pluginsCounter)
}

func (m *Model) deletePlugin(name string) {
	m.pluginsLock.Lock()
	defer m.pluginsLock.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		logger.Errorf("BUG: plugin %s not found", name)
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
		logger.Errorf("BUG: plugin %s not found", spec.Name)
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
		logger.Errorf("BUG: pipeline scheduler %s existed", spec.Name)
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
		logger.Errorf("BUG: pipeline scheduler %s not found", name)
		return
	}
	scheduler.Stop()
	scheduler.StopPipeline()
	delete(m.schedulers, name)
}

func (m *Model) updatePipeline(spec *store.PipelineSpec) {
	m.schedulersLock.Lock()
	defer m.schedulersLock.Unlock()

	scheduler, exists := m.schedulers[spec.Name]
	if !exists {
		logger.Errorf("BUG: pipeline scheduler %s not found", spec.Name)
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

func (m *Model) handlEvent(event *store.Event) {
	for name, spec := range event.Plugins {
		if spec != nil {
			if _, exists := m.plugins[name]; !exists {
				m.createPlugin(spec)
			} else {
				m.updatePlugin(spec)
			}
		} else {
			m.deletePlugin(name)
		}
	}

	for name, spec := range event.Pipelines {
		if spec != nil {
			if _, exists := m.schedulers[name]; !exists {
				m.createPipeline(spec)
			} else {
				m.updatePipeline(spec)
			}
		} else {
			m.deletePipeline(name)
		}
	}
}

func (m *Model) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	for name := range m.schedulers {
		m.deletePipeline(name)
	}
	for name := range m.plugins {
		m.deletePlugin(name)
	}
}
