package model

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
	pipelines_gw "pipelines"
)

type linearPipelineConfig struct {
	*common.PipelineCommonConfig
	WaitPluginClose bool `json:"wait_plugin_close"`
}

func linearPipelineConfigConstructor() pipelines_gw.Config {
	return &linearPipelineConfig{
		WaitPluginClose: true, // HTTPInput plugin needs this due to it has a global mux object
	}
}

//
// Linear pipeline implementation
//

type linearPipeline struct {
	conf                    *linearPipelineConfig
	ctx                     pipelines.PipelineContext
	statistics              *PipelineStatistics
	mod                     *Model
	rerunCancel, stopCancel cancelFunc
	started, stopped        int32
	rerun                   bool
	runningPlugin           string
	rerunLock               sync.RWMutex
}

func newLinearPipeline(ctx pipelines.PipelineContext, statistics *PipelineStatistics,
	conf pipelines_gw.Config, m *Model) (pipelines_gw.Pipeline, error) {

	c, ok := conf.(*linearPipelineConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *LinearPipelineConfig got %T", conf)
	}

	if ctx == nil {
		return nil, fmt.Errorf("context object is nil")
	}

	if statistics == nil {
		return nil, fmt.Errorf("statistics object is nil")
	}

	if m == nil {
		return nil, fmt.Errorf("model object is nil")
	}

	return &linearPipeline{
		ctx:        ctx,
		conf:       c,
		statistics: statistics,
		mod:        m,
	}, nil
}

func (p *linearPipeline) Name() string {
	return p.conf.PipelineName()
}

func (p *linearPipeline) Prepare() {
	pluginNames := p.conf.PluginNames()

	// Prepare all plugin first for, like, indicator exposing.
	for i := 0; i < len(pluginNames) && atomic.LoadInt32(&p.stopped) == 0; i++ {
		instance, err := p.mod.GetPluginInstance(pluginNames[i])
		if err != nil {
			// the preparation of follow plugin might depend on previous plugin
			break
		}
		instance.Prepare(p.ctx)
		p.mod.ReleasePluginInstance(instance)
	}

	p.mod.AddPluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p),
		p.cancelAndRerunRunningPlugin, common.NORMAL_PRIORITY_CALLBACK)
}

func (p *linearPipeline) Run() error {
	if atomic.LoadInt32(&p.stopped) == 1 {
		return fmt.Errorf("pipeline is stopped")
	}

	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		return fmt.Errorf("pipeline is already started")
	}

	defer atomic.StoreInt32(&p.started, 0)

	tsk := NewTask()
	var t task.Task
	t, p.stopCancel = withCancel(tsk)

	pluginNames := p.conf.PluginNames()

	startAt := time.Now()

	for i := 0; i < len(pluginNames) && atomic.LoadInt32(&p.stopped) == 0; i++ {
		// error here is acceptable to pipeline, so do not return and keep pipeline runs
		instance, err := p.mod.GetPluginInstance(pluginNames[i])
		if err != nil {
			logger.Warnf("[plugin %s get instance failed: %v]", pluginNames[i], err)
			t.SetError(err, http.StatusServiceUnavailable)
		} else {
			// Plugin might be updated during the pipeline execution.
			// This guarantees preparing the plugin instance only once.
			instance.Prepare(p.ctx)
			p.runningPlugin = pluginNames[i]
		}

		switch t.Status() {
		case task.Pending:
			tsk.start()
			fallthrough
		case task.Running:
			var (
				success, rerun bool
				done           chan struct{}
			)

			success, rerun = p.runPlugin(instance, t, tsk)

			if !success && p.conf.WaitPluginClose {
				done = make(chan struct{})

				if atomic.LoadInt32(&p.stopped) == 0 {
					plugin, _ := p.mod.GetPlugin(pluginNames[i])
					callbackName := fmt.Sprintf("%s@%p-pluginInstanceClosed@%p", p.Name(), p, instance)
					plugin.AddInstanceClosedCallback(callbackName,
						func(closedInstance plugins.Plugin) {
							if closedInstance == instance {
								plugin.DeleteInstanceClosedCallback(callbackName)
								close(done)
							}
						},
						common.NORMAL_PRIORITY_CALLBACK)
				}
			}

			p.mod.ReleasePluginInstance(instance)

			if !success && p.conf.WaitPluginClose {
				if atomic.LoadInt32(&p.stopped) == 0 {
					<-done
				}
				common.CloseChan(done)
			}

			if !success && rerun {
				i--
				continue
			}
		}

		switch t.Status() {
		case task.ResponseImmediately:
			msg := fmt.Sprintf(
				"[plugin %s in pipeline %s execution failure, resultcode=%d, error=\"%s\"]",
				pluginNames[i], p.conf.Name, t.ResultCode(), t.Error())

			if atomic.LoadInt32(&p.stopped) == 1 {
				tsk.finish(t)
			} else {
				recovered := tsk.recover(pluginNames[i], task.Running, t)
				if !recovered {
					logger.Warnf(msg)
					tsk.finish(t)
				}
			}
		case task.Finishing:
			tsk.finish(t)
		}

		if t.Status() == task.Finished {
			break
		}
	}

	if !t.Finished() {
		tsk.finish(t)
	}

	go func() {
		err1 := p.statistics.updatePipelineExecution(time.Now().Sub(startAt))
		if err1 != nil {
			logger.Errorf("[pipeline %s updates execution statistics failed: %v]", p.Name(), err1)
		}

		if t.Error() == nil {
			err1 = p.statistics.updateTaskExecution(pipelines.SuccessStatistics)
		} else {
			err1 = p.statistics.updateTaskExecution(pipelines.FailureStatistics)
		}

		if err1 != nil {
			logger.Errorf("[pipeline %s updates task execution statistics failed: %v]", p.Name(), err1)
		}
	}()

	return nil
}

func (p *linearPipeline) Close() {
	p.mod.DeletePluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p))
}

func (p *linearPipeline) Stop() {
	if atomic.LoadInt32(&p.started) == 0 {
		return // not start to run yet
	}

	if !atomic.CompareAndSwapInt32(&p.stopped, 0, 1) {
		return // already stopped
	}

	if p.stopCancel != nil {
		p.stopCancel()
	}
}

func (p *linearPipeline) runPlugin(instance plugins.Plugin, input task.Task, tsk *Task) (bool, bool) {
	p.rerunLock.Lock()
	if p.rerun {
		p.mod.DismissPluginInstance(instance)
		p.rerunLock.Unlock()
		return false, true
	}
	var i task.Task
	i, p.rerunCancel = withCancel(input)
	p.rerunLock.Unlock()

	originalCode := input.ResultCode()
	startAt := time.Now()
	err := instance.Run(p.ctx, i)
	p.runningPlugin = ""
	finishAt := time.Now()

	if !p.rerun {
		go func() {
			var kind pipelines.StatisticsKind = pipelines.AllStatistics
			if err != nil || i.Error() != nil {
				kind = pipelines.FailureStatistics
			} else {
				kind = pipelines.SuccessStatistics
			}

			err1 := p.statistics.updatePluginExecution(instance.Name(), kind, finishAt.Sub(startAt))
			if err1 != nil {
				logger.Errorf("[plugin %s updates execution statistics failed: %v]", instance.Name(), err1)
			}
		}()
	}

	if err != nil {
		if !p.rerun {
			if atomic.LoadInt32(&p.stopped) == 0 {
				logger.Warnf("[plugin %s encountered failure itself can't cover: %v]",
					instance.Name(), err)
			}

			if i.Error() == nil { // do not overwrite plugin gives error
				i.SetError(err, http.StatusServiceUnavailable)
			}
		} else {
			// clear task cancellation error
			tsk.clearError(originalCode)
		}

		if atomic.LoadInt32(&p.stopped) == 0 {
			// error caused by plugin update or execution failure
			p.mod.DismissPluginInstance(instance)
		}
	}

	rerun := p.rerun
	p.rerun = false
	p.rerunCancel = nil

	return err == nil, rerun
}

func (p *linearPipeline) cancelAndRerunRunningPlugin(updatedPlugin *Plugin) {
	p.rerunLock.RLock()

	if p.runningPlugin != updatedPlugin.Name() {
		p.rerunLock.RUnlock()
		return
	}

	p.rerunLock.RUnlock()

	p.rerunLock.Lock()
	defer p.rerunLock.Unlock()

	// DCL
	if p.runningPlugin != updatedPlugin.Name() {
		return
	}

	p.rerun = true
	if p.rerunCancel != nil {
		p.rerunCancel()
	}
}
