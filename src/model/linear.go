package model

import (
	"fmt"
	"net/http"
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
	common.PipelineCommonConfig
}

func linearPipelineConfigConstructor() pipelines_gw.Config {
	return &linearPipelineConfig{
		PipelineCommonConfig: common.PipelineCommonConfig{
			CrossPipelineRequestBacklogLength: 10240,
		},
	}
}

////

type statisticsData struct {
	startAt, finishAt time.Time
	successful        bool
}

type pluginStatisticsData struct {
	statisticsData
	pluginName string
}

//
// Linear pipeline implementation
//

type linearPipeline struct {
	conf                                    *linearPipelineConfig
	ctx                                     pipelines.PipelineContext
	statistics                              *PipelineStatistics
	mod                                     *Model
	rerunCancel, stopCancel, scheduleCancel cancelFunc
	started, stopped                        uint32
	runningPluginName                       string
	runningPluginGeneration                 uint64
	pipelineAndTaskStatChan                 chan *statisticsData
	pluginStatChan                          chan *pluginStatisticsData
	statUpdaterStop                         chan struct{}
	statUpdaterDone                         chan *struct{}
	done                                    chan struct{}
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

	pipeline := &linearPipeline{
		ctx:                     ctx,
		conf:                    c,
		statistics:              statistics,
		mod:                     m,
		rerunCancel:             NoOpCancelFunc,
		stopCancel:              NoOpCancelFunc,
		scheduleCancel:          NoOpCancelFunc,
		pipelineAndTaskStatChan: make(chan *statisticsData, 1024),
		pluginStatChan:          make(chan *pluginStatisticsData, 1024),
		statUpdaterStop:         make(chan struct{}),
		statUpdaterDone:         make(chan *struct{}),
		done:                    make(chan struct{}),
	}

	go pipeline.pipelineAndTaskStatUpdater()
	go pipeline.pluginStatUpdater()

	return pipeline, nil
}

func (p *linearPipeline) Name() string {
	return p.conf.PipelineName()
}

func (p *linearPipeline) Prepare() {
	pluginNames := p.conf.PluginNames()

	// Prepare all plugin first for, like, indicator exposing.
	for i := 0; i < len(pluginNames) && atomic.LoadUint32(&p.stopped) == 0; i++ {
		instance, _, _, err := p.mod.getPluginInstance(pluginNames[i], false)
		if err != nil {
			logger.Warnf("[plugin %s get instance failed: %v]", pluginNames[i], err)
			break // the preparation of follow plugin might depend on previous plugin
		}

		instance.Prepare(p.ctx)

		p.mod.releasePluginInstance(instance)
	}

	p.mod.AddPluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p),
		p.cancelAndRerunRunningPlugin, common.NORMAL_PRIORITY_CALLBACK)
}

func (p *linearPipeline) Run() error {
	if atomic.LoadUint32(&p.stopped) == 1 {
		return nil // pipeline is stopped before run
	}

	if !atomic.CompareAndSwapUint32(&p.started, 0, 1) {
		return fmt.Errorf("pipeline is already started")
	}

	tsk := newTask()
	var t task.Task
	t, p.stopCancel = withCancel(tsk, task.CanceledByPipelineStopped)

	pluginNames := p.conf.PluginNames()

	startAt := common.Now()
	var success, preempted, rerun bool

	for i := 0; i < len(pluginNames) && atomic.LoadUint32(&p.stopped) == 0; i++ {
		// error here is acceptable to pipeline, so do not return and keep pipeline runs
		instance, pluginType, gen, err := p.mod.getPluginInstance(pluginNames[i], true)
		if err != nil {
			logger.Warnf("[plugin %s get instance failed: %v]", pluginNames[i], err)
			t.SetError(err, http.StatusServiceUnavailable)
		}

		switch t.Status() {
		case task.Pending:
			tsk.start()
			fallthrough
		case task.Running:
			success, preempted, rerun = p.runPlugin(instance, pluginType, gen, t, tsk)

			p.mod.releasePluginInstance(instance)

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

			if atomic.LoadUint32(&p.stopped) == 1 {
				tsk.finish(t)
			} else if instance != nil {
				recovered := tsk.recover(instance.Name(), instance.Type(), task.Running, t)
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

	if !preempted && atomic.LoadUint32(&p.stopped) == 0 {
		data := &statisticsData{
			startAt:    startAt,
			finishAt:   common.Now(),
			successful: t.Error() == nil,
		}

		select {
		case p.pipelineAndTaskStatChan <- data:
		default: // skip update if busy
		}
	}

	atomic.StoreUint32(&p.started, 0)

	if atomic.LoadUint32(&p.stopped) == 1 {
		close(p.done)
	}

	return nil
}

func (p *linearPipeline) Close() {
	p.mod.DeletePluginUpdatedCallback(fmt.Sprintf("%s-cancelAndRerunRunningPlugin@%p", p.Name(), p))

	close(p.statUpdaterDone)
	close(p.pipelineAndTaskStatChan)
	close(p.pluginStatChan)

	common.CloseChan(p.done) // this is safe if close closed channel
}

func (p *linearPipeline) Stop(scheduled bool) {
	if !atomic.CompareAndSwapUint32(&p.stopped, 0, 1) {
		return // already stopped
	}

	func() {
		// to prevent p.scheduleCancel() / p.stopCancel() raises any issue in case of concurrent update/call
		defer func() {
			recover()
		}()

		if scheduled {
			p.scheduleCancel()
		} else {
			p.stopCancel()
		}
	}()

	if atomic.LoadUint32(&p.started) == 1 {
		// wait Run() exits
		<-p.done
	}

	// notify both updaters stop
	close(p.statUpdaterStop)

	// wait both updaters done
	<-p.statUpdaterDone
	<-p.statUpdaterDone
}

func (p *linearPipeline) runPlugin(instance *wrappedPlugin, pluginType plugins.PluginType, gen uint64,
	input task.Task, tsk *Task) (bool, bool, bool) {

	var t = input
	var canceller cancelFunc
	preempted := false

	if pluginType == plugins.SourcePlugin {
		// source plugin could be preempted
		t, canceller = withCancel(t, task.CanceledByPipelinePreempted)

		p.scheduleCancel = func() {
			preempted = true
			canceller()
		}
	}

	originalCode := input.ResultCode()
	rerun := false

	t, canceller = withCancel(t, task.CanceledByPluginUpdated)

	p.rerunCancel = func() {
		rerun = true
		canceller()
	}

	p.runningPluginGeneration = gen
	p.runningPluginName = instance.Name()

	startAt := common.Now()
	err := instance.Run(p.ctx, t)
	finishAt := common.Now()

	p.runningPluginName = ""
	p.runningPluginGeneration = 0
	p.rerunCancel = NoOpCancelFunc
	p.scheduleCancel = NoOpCancelFunc

	if !rerun && !preempted && atomic.LoadUint32(&p.stopped) == 0 {
		data := &pluginStatisticsData{
			statisticsData: statisticsData{
				startAt:    startAt,
				finishAt:   finishAt,
				successful: err != nil || t.Error() != nil,
			},
			pluginName: instance.Name(),
		}

		select {
		case p.pluginStatChan <- data:
		default: // skip update if busy
		}
	}

	if err != nil {
		if rerun {
			// clear task cancellation error
			tsk.clearError(originalCode)
		} else if !preempted {
			if atomic.LoadUint32(&p.stopped) == 0 {
				logger.Warnf("[plugin %s encountered failure itself can't cover: %v]", instance.Name(), err)
			}

			if t.Error() == nil { // do not overwrite plugin gives error
				t.SetError(err, http.StatusServiceUnavailable)
			}
		}

		if atomic.LoadUint32(&p.stopped) == 0 && !preempted {
			// error caused by plugin update or execution failure
			p.mod.dismissPluginInstance(instance)
		}
	}

	return err == nil, preempted, rerun
}

func (p *linearPipeline) cancelAndRerunRunningPlugin(updatedPlugin *Plugin,
	instanceDismissed bool, instanceGen uint64) {

	if p.runningPluginName != updatedPlugin.Name() || p.runningPluginGeneration > instanceGen {
		return
	}

	defer func() {
		recover() // to prevent p.rerunCancel() raises any issue in case of concurrent update/call
	}()

	p.rerunCancel()
}

func (p *linearPipeline) updatePipelineAndTaskStat(data *statisticsData) {
	err := p.statistics.updatePipelineExecution(data.finishAt.Sub(data.startAt))
	if err != nil {
		logger.Errorf("[pipeline %s updates execution statistics failed: %v]", p.Name(), err)
	}

	if data.successful {
		err = p.statistics.updateTaskExecution(pipelines.SuccessStatistics)
	} else {
		err = p.statistics.updateTaskExecution(pipelines.FailureStatistics)
	}

	if err != nil {
		logger.Errorf("[pipeline %s updates task execution statistics failed: %v]", p.Name(), err)
	}
}

func (p *linearPipeline) drainPipelineAndTaskStatData() {
	for {
		select {
		case data := <-p.pipelineAndTaskStatChan:
			p.updatePipelineAndTaskStat(data)
		default:
			return
		}
	}
}

func (p *linearPipeline) pipelineAndTaskStatUpdater() {
	for {
		select {
		case data := <-p.pipelineAndTaskStatChan:
			p.updatePipelineAndTaskStat(data)
		case <-p.statUpdaterStop:
			p.drainPipelineAndTaskStatData()
			p.statUpdaterDone <- nil // notify done
			return
		}
	}
}

func (p *linearPipeline) updatePluginStat(data *pluginStatisticsData) {
	kind := pipelines.AllStatistics
	if data.successful {
		kind = pipelines.SuccessStatistics
	} else {
		kind = pipelines.FailureStatistics
	}

	err := p.statistics.updatePluginExecution(data.pluginName, kind, data.finishAt.Sub(data.startAt))
	if err != nil {
		logger.Errorf("[plugin %s updates execution statistics failed: %v]", data.pluginName, err)
	}
}

func (p *linearPipeline) drainPluginStatData() {
	for {
		select {
		case data := <-p.pluginStatChan:
			p.updatePluginStat(data)
		default:
			return
		}
	}
}

func (p *linearPipeline) pluginStatUpdater() {
	for {
		select {
		case data := <-p.pluginStatChan:
			p.updatePluginStat(data)
		case <-p.statUpdaterStop:
			p.drainPluginStatData()
			p.statUpdaterDone <- nil // notify done
			return
		}
	}
}
