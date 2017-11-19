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

		p.mod.AddPipelineDeletedCallback(callbackName, p.cleanUpPreparedPipelineContext,
			false, "closePipelineContext")
		p.mod.AddPipelineUpdatedCallback(callbackName, p.cleanUpPreparedPipelineContext,
			false, "relaunchPipelineStage2")
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
	overwrite bool, priority string) PluginInstanceClosed {

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
			callBackName := fmt.Sprintf("%s-destroyOverduePluginInstance@%p", p.conf.PluginName(), instance)
			p.counter.AddUpdateCallback(callBackName, p.destroyOverduePluginInstance(instance))
		}
	}
}

func (p *Plugin) destroyOverduePluginInstance(instance plugins.Plugin) PluginRefCountUpdated {
	return func(plugin plugins.Plugin, count int, counter *pluginInstanceCounter) {
		if instance == plugin && count == 0 {
			p.closePluginInstance(plugin)

			callBackName := fmt.Sprintf("%s-destroyOverduePluginInstance@%p", p.conf.PluginName(), plugin)
			counter.DeleteUpdateCallback(callBackName)
		}
	}
}

func (p *Plugin) closePluginInstance(plugin plugins.Plugin) error {
	(plugin.(*wrappedPlugin)).cleanUpAndClose()

	tmp := make([]*common.NamedCallback, len(p.instanceClosedCallbacks))
	copy(tmp, p.instanceClosedCallbacks)

	for _, callback := range tmp {
		callback.Callback().(PluginInstanceClosed)(plugin)
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
		count:     make(map[plugins.Plugin]int),
		callbacks: make([]*common.NamedCallback, 0, common.CallbacksInitCapicity),
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
		callback.Callback().(PluginRefCountUpdated)(plugin, count, c)
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
		callback.Callback().(PluginRefCountUpdated)(plugin, count, c)
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
		c.callbacks, pluginName, callback, false, common.NORMAL_PRIORITY_CALLBACK)

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
