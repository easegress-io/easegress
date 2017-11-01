package model

import (
	"fmt"
	"sync"

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
	preparedPipelineContexts map[string]pipelines.PipelineContext
}

func newWrappedPlugin(mod *Model, ori plugins.Plugin) *wrappedPlugin {
	p := &wrappedPlugin{
		mod: mod,
		ori: ori,
		preparedPipelineContexts: make(map[string]pipelines.PipelineContext),
	}

	callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenPipelineUpdatedOrDeleted@%p", p.Name(), p)
	go p.mod.AddPipelineDeletedCallback(callbackName, p.cleanUpPreparedPipelineContext,
		false, common.CriticalCallback)
	go p.mod.AddPipelineUpdatedCallback(callbackName, p.cleanUpPreparedPipelineContext,
		false, common.CriticalCallback)

	return p
}

func (p *wrappedPlugin) Prepare(ctx pipelines.PipelineContext) {
	// booking
	_, ok := p.preparedPipelineContexts[ctx.PipelineName()]
	if ok {
		logger.Errorf("[BUG: plugin %s is prepared on the same pipeline %s twice, overwrite]",
			p.Name(), ctx.PipelineName())
	}

	p.preparedPipelineContexts[ctx.PipelineName()] = ctx

	logger.Debugf("[prepare plugin %s for pipeline %s]", p.Name(), ctx.PipelineName())
	p.ori.Prepare(ctx)
	logger.Debugf("[plugin %s prepared for pipeline %s]", p.Name(), ctx.PipelineName())
}

func (p *wrappedPlugin) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
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
	callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenPipelineUpdatedOrDeleted@%p", p.Name(), p)
	go p.mod.DeletePipelineDeletedCallback(callbackName)
	go p.mod.DeletePipelineUpdatedCallback(callbackName)

	logger.Debugf("[closing plugin %s]", p.Name())
	p.ori.Close()
	logger.Debugf("[closed plugin %s]", p.Name())
}

func (p *wrappedPlugin) cleanUpAndClose() {
	for _, ctx := range p.preparedPipelineContexts {
		p.CleanUp(ctx)
	}

	// clear the book
	for pipelineName := range p.preparedPipelineContexts {
		delete(p.preparedPipelineContexts, pipelineName)
	}

	p.Close()
}

func (p *wrappedPlugin) cleanUpPreparedPipelineContext(pipeline *Pipeline) {
	ctx, ok := p.preparedPipelineContexts[pipeline.Name()]
	if !ok {
		// the plugin was not prepared on the pipeline
		return
	}

	delete(p.preparedPipelineContexts, pipeline.Name())

	p.CleanUp(ctx)
}

//
// Plugin entry in model structure
//

type PluginInstanceClosed func()

type Plugin struct {
	sync.RWMutex
	typ                     string
	conf                    plugins.Config
	constructor             plugins.Constructor
	counter                 *pluginInstanceCounter
	instance                *wrappedPlugin
	instanceClosedCallbacks []*common.NamedCallback
}

func newPlugin(typ string, conf plugins.Config,
	constructor plugins.Constructor, counter *pluginInstanceCounter) *Plugin {

	return &Plugin{
		conf:        conf,
		typ:         typ,
		constructor: constructor,
		counter:     counter,
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

func (p *Plugin) GetInstance(mod *Model) (plugins.Plugin, error) {
	p.RLock()
	instance := p.instance
	p.RUnlock()

	if instance != nil {
		return p.instance, nil
	}

	p.Lock()
	defer p.Unlock()

	// DCL
	if p.instance != nil {
		return p.instance, nil
	}

	ori, err := p.constructor(p.conf)
	if err != nil {
		return nil, err
	}

	instance = newWrappedPlugin(mod, ori)
	p.instance = instance
	return instance, nil
}

func (p *Plugin) AddInstanceClosedCallback(name string, callback PluginInstanceClosed,
	overwrite bool, priority common.CallbackPriority) PluginInstanceClosed {

	p.Lock()
	defer p.Unlock()

	var oriCallback interface{}
	p.instanceClosedCallbacks, oriCallback, _ = common.AddCallback(
		p.instanceClosedCallbacks, name, callback, overwrite, priority)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginInstanceClosed)
	}
}

func (p *Plugin) DeleteInstanceClosedCallback(name string) PluginInstanceClosed {
	p.Lock()
	defer p.Unlock()

	var oriCallback interface{}
	p.instanceClosedCallbacks, oriCallback = common.DeleteCallback(p.instanceClosedCallbacks, name)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginInstanceClosed)
	}
}

func (p *Plugin) DismissInstance() {
	p.Lock()
	defer p.Unlock()
	p.dismissInstance()
}

func (p *Plugin) UpdateConfig(conf plugins.Config) {
	p.Lock()
	defer p.Unlock()

	p.conf = conf
	p.dismissInstance()
}

func (p *Plugin) dismissInstance() {
	instance := p.instance
	p.instance = nil

	if instance != nil {
		_, ok := p.counter.CompareRefAndFunc(
			instance, 0, func() error { return p.closePluginInstance(instance) })
		if !ok {
			p.counter.AddUpdateCallback(p.conf.PluginName(), p.destroyOverduePluginInstance)
		}
	}
}

func (p *Plugin) destroyOverduePluginInstance(plugin plugins.Plugin, count int, counter *pluginInstanceCounter) {
	if count == 0 {
		p.closePluginInstance(plugin.(*wrappedPlugin))
		counter.DeleteUpdateCallback(p.conf.PluginName())
	}
}

func (p *Plugin) closePluginInstance(instance *wrappedPlugin) error {
	instance.cleanUpAndClose()

	tmp := make([]*common.NamedCallback, len(p.instanceClosedCallbacks))
	copy(tmp, p.instanceClosedCallbacks)

	for _, callback := range tmp {
		callback.Callback().(PluginInstanceClosed)()
	}

	return nil
}

//
// Plugin reference counter
//

type PluginRefCountUpdated func(plugin plugins.Plugin, count int, counter *pluginInstanceCounter)

type pluginInstanceCounter struct {
	sync.Mutex
	count     map[plugins.Plugin]int
	callbacks []*common.NamedCallback
}

func newPluginRefCounter() *pluginInstanceCounter {
	return &pluginInstanceCounter{
		count: make(map[plugins.Plugin]int),
	}
}

func (c *pluginInstanceCounter) AddRef(plugin plugins.Plugin) int {
	c.Lock()
	count := c.count[plugin]
	count += 1
	c.count[plugin] = count

	tmp := make([]*common.NamedCallback, len(c.callbacks))
	copy(tmp, c.callbacks)
	c.Unlock()

	for _, callback := range tmp {
		if callback.Name() == plugin.Name() {
			callback.Callback().(PluginRefCountUpdated)(plugin, count, c)
		}
	}

	return count
}

func (c *pluginInstanceCounter) DeleteRef(plugin plugins.Plugin) int {
	c.Lock()
	count, exists := c.count[plugin]
	if !exists || count == 0 {
		c.Unlock()
		return -1
	}

	count -= 1
	c.count[plugin] = count

	tmp := make([]*common.NamedCallback, len(c.callbacks))
	copy(tmp, c.callbacks)
	c.Unlock()

	for _, callback := range tmp {
		if callback.Name() == plugin.Name() {
			callback.Callback().(PluginRefCountUpdated)(plugin, count, c)
		}
	}

	return count
}

func (c *pluginInstanceCounter) CompareRefAndFunc(plugin plugins.Plugin, count int, fun func() error) (error, bool) {
	c.Lock()
	defer c.Unlock()

	count1, exists := c.count[plugin]
	if exists && count1 == count {
		return fun(), true
	}

	return nil, false
}

func (c *pluginInstanceCounter) AddUpdateCallback(pluginName string,
	callback PluginRefCountUpdated) PluginRefCountUpdated {

	c.Lock()
	defer c.Unlock()

	var oriCallback interface{}
	c.callbacks, oriCallback, _ = common.AddCallback(
		c.callbacks, pluginName, callback, false, common.NormalCallback)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginRefCountUpdated)
	}
}

func (c *pluginInstanceCounter) DeleteUpdateCallback(pluginName string) PluginRefCountUpdated {
	c.Lock()
	defer c.Unlock()

	var oriCallback interface{}
	c.callbacks, oriCallback = common.DeleteCallback(c.callbacks, pluginName)

	if oriCallback == nil {
		return nil
	} else {
		return oriCallback.(PluginRefCountUpdated)
	}
}
