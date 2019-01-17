package model

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/store"
	"github.com/megaease/easegateway/pkg/logger"

	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/plugins"
	"github.com/megaease/easegateway/pkg/task"
)

//
// Plugin wrapper for plugin close and cleanup
//

type pluginInstanceClosed func()

type wrappedPlugin struct {
	mod                      *Model
	ori                      plugins.Plugin
	typ                      string
	generation               uint64
	preparationLock          sync.RWMutex
	preparedPipelineContexts map[string]pipelines.PipelineContext
	closedCallbacks          *common.NamedCallbackSet
}

func newWrappedPlugin(mod *Model, ori plugins.Plugin, typ string, gen uint64) *wrappedPlugin {
	plugin := &wrappedPlugin{
		mod:                      mod,
		ori:                      ori,
		typ:                      typ,
		generation:               gen,
		preparedPipelineContexts: make(map[string]pipelines.PipelineContext),
		closedCallbacks:          common.NewNamedCallbackSet(),
	}

	logger.Debugf("created plugin %s at %p", plugin.Name(), ori)

	return plugin
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

	logger.Debugf("prepare plugin %s for pipeline %s (plugin=%p, context=%p)",
		p.Name(), ctx.PipelineName(), p.ori, ctx)

	p.ori.Prepare(ctx)

	logger.Debugf("plugin %s prepared for pipeline %s (plugin=%p, context=%p)",
		p.Name(), ctx.PipelineName(), p.ori, ctx)

	p.preparedPipelineContexts[ctx.PipelineName()] = ctx

	pc, ok := ctx.(*pipelineContext)
	if ok {
		callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenContextClosing@%p", p.Name(), p)
		pc.addClosingCallback(callbackName, p.cleanUpPreparedPipelineContext, common.NORMAL_PRIORITY_CALLBACK)
	}
}

func (p *wrappedPlugin) Run(ctx pipelines.PipelineContext, t task.Task) error {
	return p.ori.Run(ctx, t)
}

func (p *wrappedPlugin) Name() string {
	return p.ori.Name()
}

func (p *wrappedPlugin) Type() string {
	return p.typ
}

func (p *wrappedPlugin) CleanUp(ctx pipelines.PipelineContext) {
	pc, ok := ctx.(*pipelineContext)
	if ok {
		callbackName := fmt.Sprintf("%s-cleanUpPreparedPipelineContextMapWhenContextClosing@%p", p.Name(), p)
		pc.deleteClosingCallback(callbackName)
	}

	logger.Debugf("cleaning plugin %s up from pipeline %s (plugin=%p, context=%p)",
		p.Name(), ctx.PipelineName(), p.ori, ctx)

	p.ori.CleanUp(ctx)

	logger.Debugf("cleaned plugin %s up from pipeline %s (plugin=%p, context=%p)",
		p.Name(), ctx.PipelineName(), p.ori, ctx)
}

func (p *wrappedPlugin) Close() {
	logger.Debugf("closing plugin %s at %p", p.Name(), p.ori)
	p.ori.Close()
	logger.Debugf("closed plugin %s at %p", p.Name(), p.ori)

	for _, callback := range p.closedCallbacks.GetCallbacks() {
		callback.Callback().(pluginInstanceClosed)()
	}
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

func (p *wrappedPlugin) cleanUpPreparedPipelineContext(ctx pipelines.PipelineContext) {
	p.preparationLock.RLock()

	ctx, exists := p.preparedPipelineContexts[ctx.PipelineName()]
	if !exists {
		// the plugin was not prepared on the pipeline
		p.preparationLock.RUnlock()
		return
	}

	p.preparationLock.RUnlock()

	p.preparationLock.Lock()

	// DCL
	ctx, exists = p.preparedPipelineContexts[ctx.PipelineName()]
	if !exists {
		p.preparationLock.Unlock()
		return
	}

	delete(p.preparedPipelineContexts, ctx.PipelineName())

	p.preparationLock.Unlock()

	p.CleanUp(ctx)
}

func (p *wrappedPlugin) addClosedCallback(name string, callback pluginInstanceClosed, priority string) {
	p.closedCallbacks = common.AddCallback(p.closedCallbacks, name, callback, priority)
}

func (p *wrappedPlugin) deleteClosedCallback(name string) {
	p.closedCallbacks = common.DeleteCallback(p.closedCallbacks, name)
}

//
// Plugin entry in model structure
//

type Plugin struct {
	sync.RWMutex
	typ                string
	conf               plugins.Config
	constructor        plugins.Constructor
	counter            *pluginInstanceCounter
	instance           *wrappedPlugin
	pluginType         plugins.PluginType
	singleton          bool
	singletonClosed    chan struct{}
	currentInstanceGen uint64
}

func newPlugin(spec *store.PluginSpec, counter *pluginInstanceCounter) *Plugin {

	return &Plugin{
		conf:        spec.Config.(plugins.Config),
		typ:         spec.Type,
		constructor: spec.Constructor,
		counter:     counter,
		pluginType:  plugins.UnknownType,
	}
}

func (p *Plugin) Name() string {
	p.RLock()
	defer p.RUnlock()
	return p.conf.PluginName()
}

func (p *Plugin) Type() string {
	return p.typ
}

func (p *Plugin) Config() plugins.Config {
	p.RLock()
	defer p.RUnlock()
	return p.conf
}

func (p *Plugin) GetInstance(mod *Model, prepareForNew bool) (*wrappedPlugin, plugins.PluginType, uint64, error) {
	p.RLock()
	if p.instance != nil {
		p.RUnlock()
		return p.instance, p.pluginType, atomic.LoadUint64(&p.currentInstanceGen), nil
	}

	p.RUnlock()

	p.Lock()
	defer p.Unlock()

	// DCL
	if p.instance != nil {
		return p.instance, p.pluginType, atomic.LoadUint64(&p.currentInstanceGen), nil
	}

	if p.singleton { // block and wait old plugin instances close completely
		<-p.singletonClosed
	}

	ori, pluginType, singleton, err := p.constructor(p.conf)
	if err != nil {
		return nil, pluginType, 0, err
	}

	instance := newWrappedPlugin(mod, ori, p.Type(), atomic.AddUint64(&p.currentInstanceGen, 1))
	p.instance = instance
	p.pluginType = pluginType
	p.singleton = singleton
	if p.singleton {
		p.singletonClosed = make(chan struct{})
	}

	if prepareForNew {
		// prepare new plugin instance on pipelines
		mod.preparePluginInstance(instance)
	}

	return instance, pluginType, atomic.LoadUint64(&p.currentInstanceGen), nil
}

func (p *Plugin) DismissInstance(instance *wrappedPlugin) (bool, uint64) {
	p.Lock()
	defer p.Unlock()
	return p.dismissInstance(instance)
}

func (p *Plugin) UpdateConfig(conf plugins.Config) (bool, uint64) {
	p.Lock()
	defer p.Unlock()

	p.conf = conf
	return p.dismissInstance(nil)
}

func (p *Plugin) dismissInstance(instance *wrappedPlugin) (bool, uint64) {
	inst := p.instance

	if instance != nil && inst != instance {
		return false, 0
	}

	p.instance = nil

	if inst != nil {
		if p.singleton {
			inst.addClosedCallback("pluginInstanceClosed",
				func() { close(p.singletonClosed) }, // let p.GetInstance() continues
				common.NORMAL_PRIORITY_CALLBACK)
		}

		p.counter.CompareRefAndFunc(inst, 0,
			func() { inst.cleanUpAndClose() },
			func() {
				go func() {
					for {
						count, err := p.counter.RefCount(inst)
						if err != nil {
							logger.Errorf("BUG: query reference count of plugin %s instance failed: %v",
								inst.Name(), err)
							break
						}

						if count == 0 {
							break
						}

						logger.Debugf("spin to wait old plugin instance closes")
						time.Sleep(time.Millisecond)
					}

					inst.cleanUpAndClose()
				}()
			})

		return true, inst.generation
	}

	return false, 0
}

//
// Plugin reference counter
//

type PluginRefCountUpdated func(plugin plugins.Plugin, count int64, counter *pluginInstanceCounter)

type pluginInstanceCounter struct {
	sync.RWMutex
	count map[*wrappedPlugin]*int64
}

func newPluginRefCounter() *pluginInstanceCounter {
	return &pluginInstanceCounter{
		count: make(map[*wrappedPlugin]*int64),
	}
}

func (c *pluginInstanceCounter) AddRef(plugin *wrappedPlugin) int64 {
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

	return atomic.AddInt64(count, 1)
}

func (c *pluginInstanceCounter) DeleteRef(plugin *wrappedPlugin) int64 {
	c.RLock()
	count, exists := c.count[plugin]
	if !exists || atomic.LoadInt64(count) < 1 {
		c.RUnlock()
		return -1
	}
	c.RUnlock()

	return atomic.AddInt64(count, -1)
}

func (c *pluginInstanceCounter) RefCount(plugin *wrappedPlugin) (int64, error) {
	c.RLock()
	count, exists := c.count[plugin]
	c.RUnlock()

	if !exists {
		return -1, fmt.Errorf("plugin not found")
	}

	return atomic.LoadInt64(count), nil
}

func (c *pluginInstanceCounter) CompareRefAndFunc(plugin *wrappedPlugin, count int64,
	hitFunc func(), missFunc func()) (bool, error) {

	count1, err := c.RefCount(plugin)
	if err != nil {
		return false, err
	}

	if count1 == count {
		hitFunc()
		return true, nil
	} else {
		missFunc()
		return false, nil
	}
}
