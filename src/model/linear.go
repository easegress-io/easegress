package model

import (
	"fmt"
	"net/http"
	"sync"
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
	sync.Mutex
	conf                    *linearPipelineConfig
	ctx                     pipelines.PipelineContext
	statistics              *PipelineStatistics
	mod                     *Model
	rerunCancel, stopCancel cancelFunc
	started, stopped, rerun bool
	runningPlugin           string
	rerunLock               sync.Mutex
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

func (p *linearPipeline) Run() error {
	p.Lock()
	if p.started {
		p.Unlock()
		return fmt.Errorf("pipeline is already started")
	}

	if p.stopped {
		p.Unlock()
		return fmt.Errorf("pipeline is stopped")
	}

	p.started = true
	defer func() {
		p.started = false
	}()

	tsk := NewTask()
	var t task.Task
	t, p.stopCancel = withCancel(tsk)
	p.Unlock()

	// Prepare all plugin first for, like, indicator exposing.
	pluginNames := p.conf.PluginNames()
	for i := 0; i < len(pluginNames) && !p.stopped; i++ {
		instance, err := p.mod.GetPluginInstance(pluginNames[i])
		if err == nil {
			p.ctx.PreparePlugin(pluginNames[i], func() { instance.Prepare(p.ctx) })
			p.mod.ReleasePluginInstance(instance)
		}
	}

	p.mod.AddPluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p),
		p.cancelAndRerunRunningPlugin, false)

	defer func() {
		p.mod.DeletePluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p))
	}()

	startAt := time.Now()

	for i := 0; i < len(pluginNames) && !p.stopped; i++ {
		// error here is acceptable to pipeline, so do not return and keep pipeline runs
		instance, err := p.mod.GetPluginInstance(pluginNames[i])
		if err != nil {
			logger.Warnf("[plugin %s get instance failed: %v]", pluginNames[i], err)
			t.SetError(err, http.StatusServiceUnavailable)
		} else {
			// Plugin might be updated during the pipeline execution.
			// Pipeline context guarantees preparing the plugin instance only once.
			p.ctx.PreparePlugin(pluginNames[i], func() { instance.Prepare(p.ctx) })

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

			t, success, rerun = p.runPlugin(instance, t, tsk)

			if !success && p.conf.WaitPluginClose {
				done = make(chan struct{})

				if !p.stopped {
					plugin := p.mod.GetPlugin(pluginNames[i])
					plugin.AddInstanceClosedCallback(
						fmt.Sprintf("%s-pluginInstanceClosed@%p", p.Name(), p),
						func() {
							plugin.DeleteInstanceClosedCallback(
								fmt.Sprintf("%s-pluginInstanceClosed@%p", p.Name(), p))
							close(done)
						},
						false)
				}
			}

			p.mod.ReleasePluginInstance(instance)

			if !success && p.conf.WaitPluginClose {
				if !p.stopped {
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

			if p.stopped {
				tsk.finish(t)
			} else {
				recovered, t1 := tsk.recover(pluginNames[i], task.Running, t)
				if recovered {
					t = t1
				} else {
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

	err1 := p.statistics.updatePipelineExecution(time.Now().Sub(startAt))
	if err1 != nil {
		logger.Errorf("[pipeline %s updates execution statistics failed: %v]", p.Name(), err1)
	}

	if t.Error() == nil {
		p.statistics.updateTaskExecution(pipelines.SuccessStatistics)
	} else {
		p.statistics.updateTaskExecution(pipelines.FailureStatistics)
	}

	return nil
}

func (p *linearPipeline) Close() {
	// Nothing to do.
}

func (p *linearPipeline) Stop() {
	p.Lock()
	defer p.Unlock()

	if !p.started || p.stopped {
		return
	}

	p.stopped = true

	if p.stopCancel != nil {
		p.stopCancel()
	}
}

func (p *linearPipeline) runPlugin(instance plugins.Plugin, input task.Task, tsk *Task) (task.Task, bool, bool) {
	p.rerunLock.Lock()
	if p.rerun {
		p.mod.DismissPluginInstance(instance.Name())
		p.rerunLock.Unlock()
		return input, false, true
	}
	var i task.Task
	i, p.rerunCancel = withCancel(input)
	p.rerunLock.Unlock()

	originalCode := input.ResultCode()
	startAt := time.Now()
	output, err := instance.Run(p.ctx, i)
	p.runningPlugin = ""
	finishAt := time.Now()

	if p.rerun {
		output = nil
	}

	if output == nil {
		if !p.rerun {
			logger.Warnf("[plugin %s doesn't output task, use input task continually.]", instance.Name())
		}
		output = input
	}

	if !p.rerun {
		var kind pipelines.StatisticsKind = pipelines.AllStatistics
		if err != nil || output.Error() != nil {
			kind = pipelines.FailureStatistics
		} else {
			kind = pipelines.SuccessStatistics
		}

		err1 := p.statistics.updatePluginExecution(instance.Name(), kind, finishAt.Sub(startAt))
		if err1 != nil {
			logger.Errorf("[plugin %s updates execution statistics failed: %v]", instance.Name(), err1)
		}
	}

	if err != nil {
		if !p.rerun {
			if !p.stopped {
				logger.Warnf("[plugin %s encountered failure itself can't cover: %v]",
					instance.Name(), err)
			}

			if output.Error() == nil { // do not overwrite plugin gives error
				output.SetError(err, http.StatusServiceUnavailable)
			}
		} else {
			// clear task cancellation error
			tsk.clearError(originalCode)
		}

		if !p.stopped {
			// error caused by plugin update or execution failure
			p.mod.DismissPluginInstance(instance.Name())
		}
	}

	rerun := p.rerun
	p.rerun = false
	p.rerunCancel = nil

	return output, err == nil, rerun
}

func (p *linearPipeline) cancelAndRerunRunningPlugin(updatedPlugin *Plugin) {
	p.rerunLock.Lock()
	defer p.rerunLock.Unlock()

	if p.runningPlugin != updatedPlugin.Name() {
		return
	}

	p.rerun = true
	if p.rerunCancel != nil {
		p.rerunCancel()
	}
}
