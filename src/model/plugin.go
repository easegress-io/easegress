package model

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
)

//
// Plugin wrapper for plugin close and cleanup
//

type wrappedPlugin struct {
	mod                      *Model
	ori                      plugins.Plugin
	preparationLock          sync.RWMutex
	preparedPipelineContexts map[string]pipelines.PipelineContext
}

func newWrappedPlugin(mod *Model, ori plugins.Plugin) *wrappedPlugin {
	p := &wrappedPlugin{
		mod: mod,
		ori: ori,
		preparedPipelineContexts: make(map[string]pipelines.PipelineContext),
	}

	go func() {
		callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenPipelineUpdatedOrDeleted@%p", p.Name(), p)

		p.mod.AddPipelineDeletedCallback(callbackName, p.cleanUpPreparedPipelineContext, "closePipelineContext")
		p.mod.AddPipelineUpdatedCallback(callbackName, p.cleanUpPreparedPipelineContext, "relaunchPipelineStage2")
	}()

	return p
}

func (p *wrappedPlugin) Prepare(ctx pipelines.PipelineContext) {
	p.preparationLock.RLock()
	_, exists := p.preparedPipelineContexts[ctx.PipelineName()]
	if exists {
		p.preparationLock.RUnlock()
		return
	}
	p.preparationLock.RUnlock()

	p.preparationLock.Lock()
	defer p.preparationLock.Unlock()

	// DCL
	_, exists = p.preparedPipelineContexts[ctx.PipelineName()]
	if exists {
		return
	}

	logger.Debugf("[prepare plugin %s for pipeline %s]", p.Name(), ctx.PipelineName())
	p.ori.Prepare(ctx)
	logger.Debugf("[plugin %s prepared for pipeline %s]", p.Name(), ctx.PipelineName())

	p.preparedPipelineContexts[ctx.PipelineName()] = ctx
}

func (p *wrappedPlugin) Run(ctx pipelines.PipelineContext, t task.Task) error {
	return p.ori.Run(ctx, t)
}

func (p *wrappedPlugin) Name() string {
	return p.ori.Name()
}

func (p *wrappedPlugin) CleanUp(ctx pipelines.PipelineContext) {
	logger.Debugf("[cleaning plugin %s up from pipeline %s]", p.Name(), ctx.PipelineName())
	p.ori.CleanUp(ctx)
	logger.Debugf("[cleaned plugin %s up from pipeline %s]", p.Name(), ctx.PipelineName())
}

func (p *wrappedPlugin) Close() {
	go func() {
		callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenPipelineUpdatedOrDeleted@%p", p.Name(), p)

		p.mod.DeletePipelineDeletedCallback(callbackName)
		p.mod.DeletePipelineUpdatedCallback(callbackName)
	}()

	logger.Debugf("[closing plugin %s]", p.Name())
	p.ori.Close()
	logger.Debugf("[closed plugin %s]", p.Name())
}

func (p *wrappedPlugin) cleanUpAndClose() {
	p.preparationLock.Lock()

	for _, ctx := range p.preparedPipelineContexts {
		p.CleanUp(ctx)
	}

	// clear the book
	for pipelineName := range p.preparedPipelineContexts {
		delete(p.preparedPipelineContexts, pipelineName)
	}

	p.preparationLock.Unlock()

	p.Close()
}

func (p *wrappedPlugin) cleanUpPreparedPipelineContext(pipeline *Pipeline) {
	p.preparationLock.RLock()

	ctx, exists := p.preparedPipelineContexts[pipeline.Name()]
	if !exists {
		// the plugin was not prepared on the pipeline
		p.preparationLock.RUnlock()
		return
	}

	p.preparationLock.RUnlock()

	p.preparationLock.Lock()

	// DCL
	ctx, exists = p.preparedPipelineContexts[pipeline.Name()]
	if !exists {
		p.preparationLock.Unlock()
		return
	}

	delete(p.preparedPipelineContexts, pipeline.Name())

	p.preparationLock.Unlock()

	p.CleanUp(ctx)
}

//
// Plugin entry in model structure
//

type PluginInstanceClosed func(instance plugins.Plugin)

type Plugin struct {
	sync.RWMutex
	typ                     string
	conf                    plugins.Config
	constructor             plugins.Constructor
	counter                 *pluginInstanceCounter
	instance                *wrappedPlugin
	pluginType              plugins.PluginType
	instanceClosedCallbacks *common.NamedCallbackSet
}

func newPlugin(typ string, conf plugins.Config,
	constructor plugins.Constructor, counter *pluginInstanceCounter) *Plugin {

	return &Plugin{
		conf:                    conf,
		typ:                     typ,
		constructor:             constructor,
		counter:                 counter,
		pluginType:              plugins.UnknownType,
		instanceClosedCallbacks: common.NewNamedCallbackSet(),
	}
}

func (p *Plugin) Name() string {
	p.RLock()
	defer p.RUnlock()
	return p.conf.PluginName()
}

func (p *Plugin) Type() string {
	p.RLock()
	defer p.RUnlock()
	return p.typ
}

func (p *Plugin) Config() plugins.Config {
	p.RLock()
	defer p.RUnlock()
	return p.conf
}

func (p *Plugin) GetInstance(mod *Model) (plugins.Plugin, plugins.PluginType, error) {
	p.RLock()
	if p.instance != nil {
		p.RUnlock()
		return p.instance, p.pluginType, nil
	}

	p.RUnlock()

	p.Lock()
	defer p.Unlock()

	// DCL
	if p.instance != nil {
		return p.instance, p.pluginType, nil
	}

	ori, pluginType, err := p.constructor(p.conf)
	if err != nil {
		return nil, pluginType, err
	}

	instance := newWrappedPlugin(mod, ori)
	p.instance = instance
	p.pluginType = pluginType
	return instance, pluginType, nil
}

func (p *Plugin) AddInstanceClosedCallback(name string, callback PluginInstanceClosed, priority string) {
	p.Lock()
	p.instanceClosedCallbacks = common.AddCallback(p.instanceClosedCallbacks, name, callback, priority)
	p.Unlock()
}

func (p *Plugin) DeleteInstanceClosedCallback(name string) {
	p.Lock()
	p.instanceClosedCallbacks = common.DeleteCallback(p.instanceClosedCallbacks, name)
	p.Unlock()
}

func (p *Plugin) DismissInstance(instance plugins.Plugin) {
	p.Lock()
	p.dismissInstance(instance)
	p.Unlock()
}

func (p *Plugin) UpdateConfig(conf plugins.Config) {
	p.Lock()
	defer p.Unlock()

	p.conf = conf
	p.dismissInstance(nil)
}

func (p *Plugin) dismissInstance(instance plugins.Plugin) {
	inst := p.instance

	if instance != nil && inst != instance {
		return
	}

	p.instance = nil

	if inst != nil {
		_, ok := p.counter.CompareRefAndFunc(
			inst, 0, func() error { return p.closePluginInstance(inst) })
		if !ok {
			callBackName := fmt.Sprintf("%s-destroyOverduePluginInstance@%p", p.conf.PluginName(), inst)
			p.counter.AddUpdateCallback(callBackName, p.destroyOverduePluginInstance(inst))
		}
	}
}

func (p *Plugin) destroyOverduePluginInstance(instance plugins.Plugin) PluginRefCountUpdated {
	return func(plugin plugins.Plugin, count int64, counter *pluginInstanceCounter) {
		if instance == plugin && count == 0 {
			p.closePluginInstance(plugin)

			callBackName := fmt.Sprintf("%s-destroyOverduePluginInstance@%p", p.conf.PluginName(), plugin)
			counter.DeleteUpdateCallback(callBackName)
		}
	}
}

func (p *Plugin) closePluginInstance(plugin plugins.Plugin) error {
	(plugin.(*wrappedPlugin)).cleanUpAndClose()

	tmp := p.instanceClosedCallbacks.CopyCallbacks()

	for _, callback := range tmp {
		callback.Callback().(PluginInstanceClosed)(plugin)
	}

	return nil
}

//
// Plugin reference counter
//

type PluginRefCountUpdated func(plugin plugins.Plugin, count int64, counter *pluginInstanceCounter)

type pluginInstanceCounter struct {
	sync.RWMutex
	count     map[plugins.Plugin]*int64
	callbacks *common.NamedCallbackSet
}

func newPluginRefCounter() *pluginInstanceCounter {
	return &pluginInstanceCounter{
		count:     make(map[plugins.Plugin]*int64),
		callbacks: common.NewNamedCallbackSet(),
	}
}

func (c *pluginInstanceCounter) AddRef(plugin plugins.Plugin) int64 {
	c.RLock()
	count, exists := c.count[plugin]
	c.RUnlock()

	if !exists {
		c.Lock()
		// DCL
		count, exists = c.count[plugin]
		if !exists {
			var v int64
			count = &v
			c.count[plugin] = count
		}
		c.Unlock()
	}

	value := atomic.AddInt64(count, 1)

	c.RLock()
	tmp := c.callbacks.CopyCallbacks()
	c.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginRefCountUpdated)(plugin, value, c)
	}

	return value
}

func (c *pluginInstanceCounter) DeleteRef(plugin plugins.Plugin) int64 {
	c.RLock()
	count, exists := c.count[plugin]
	if !exists || atomic.LoadInt64(count) < 1 {
		c.RUnlock()
		return -1
	}
	c.RUnlock()

	value := atomic.AddInt64(count, -1)

	c.RLock()
	tmp := c.callbacks.CopyCallbacks()
	c.RUnlock()

	for _, callback := range tmp {
		callback.Callback().(PluginRefCountUpdated)(plugin, value, c)
	}

	return value
}

func (c *pluginInstanceCounter) CompareRefAndFunc(plugin plugins.Plugin, count int64, fun func() error) (error, bool) {
	c.RLock()
	count1, exists := c.count[plugin]
	c.RUnlock()

	if exists && atomic.LoadInt64(count1) == count {
		return fun(), true
	}

	return nil, false
}

func (c *pluginInstanceCounter) AddUpdateCallback(pluginName string, callback PluginRefCountUpdated) {
	c.Lock()
	c.callbacks = common.AddCallback(c.callbacks, pluginName, callback, common.NORMAL_PRIORITY_CALLBACK)
	c.Unlock()
}

func (c *pluginInstanceCounter) DeleteUpdateCallback(pluginName string) {
	c.Lock()
	c.callbacks = common.DeleteCallback(c.callbacks, pluginName)
	c.Unlock()
}
