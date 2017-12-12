package engine

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"

	"logger"
	"model"
	"option"
	pipelines_gw "pipelines"
)

type pipelineInstance struct {
	instance pipelines_gw.Pipeline
	stop     chan struct{}
	done     chan struct{}
}

func newPipelineInstance(instance pipelines_gw.Pipeline) *pipelineInstance {
	return &pipelineInstance{
		instance: instance,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (pi *pipelineInstance) prepare() {
	pi.instance.Prepare()
}

func (pi *pipelineInstance) run() {
loop:
	for {
		select {
		case <-pi.stop:
			break loop
		default:
			err := pi.instance.Run()
			if err != nil {
				logger.Errorf(
					"[pipeline %s runs error and exits exceptionally: %v]",
					pi.instance.Name(), err)
				break loop
			}
		}
	}

	pi.instance.Close()
	close(pi.done)
}

func (pi *pipelineInstance) terminate(scheduled bool) chan struct{} {
	pi.instance.Stop(scheduled)
	close(pi.stop)
	return pi.done
}

////

type pipelineScheduler interface {
	PipelineName() string
	SourceInputTrigger() pipelines.SourceInputTrigger
	Start(ctx pipelines.PipelineContext, statistics *model.PipelineStatistics, mod *model.Model)
	Stop()
	StopPipeline()
}

////

const PIPELINE_STOP_TIMEOUT_SECONDS = 30

type commonPipelineScheduler struct {
	pipeline         *model.Pipeline
	instancesLock    sync.RWMutex
	instances        []*pipelineInstance
	started, stopped uint32
}

func newCommonPipelineScheduler(pipeline *model.Pipeline) *commonPipelineScheduler {
	return &commonPipelineScheduler{
		pipeline: pipeline,
	}
}

func (scheduler *commonPipelineScheduler) PipelineName() string {
	return scheduler.pipeline.Name()
}

func (scheduler *commonPipelineScheduler) startPipeline(parallelism uint32,
	ctx pipelines.PipelineContext, statistics *model.PipelineStatistics, mod *model.Model) (uint32, uint32) {

	if parallelism == 0 { // defensive
		parallelism = 1
	}

	scheduler.instancesLock.Lock()
	defer scheduler.instancesLock.Unlock()

	currentParallelism := uint32(len(scheduler.instances))

	if atomic.LoadUint32(&scheduler.stopped) == 1 ||
		currentParallelism == ^uint32(0) { // 4294967295
		return currentParallelism, 0 // scheduler is stop or reach the cap
	}

	left := option.PipelineMaxParallelism - currentParallelism
	if parallelism > left {
		parallelism = left
	}

	idx := uint32(0)
	for idx < parallelism {
		pipeline, err := scheduler.pipeline.GetInstance(ctx, statistics, mod)
		if err != nil {
			logger.Errorf("[launch pipeline %s-#%d failed: %v]",
				scheduler.PipelineName(), currentParallelism+idx+1, err)

			return currentParallelism, idx
		}

		instance := newPipelineInstance(pipeline)
		scheduler.instances = append(scheduler.instances, instance)
		currentParallelism++

		instance.prepare()
		go instance.run()

		idx++
	}

	return currentParallelism, idx
}

func (scheduler *commonPipelineScheduler) stopPipelineInstance(idx int, instance *pipelineInstance, scheduled bool) {
	select {
	case <-instance.terminate(scheduled): // wait until stop
	case <-time.After(PIPELINE_STOP_TIMEOUT_SECONDS * time.Second):
		logger.Warnf("[stopped pipeline %s instance #%d timeout (%d seconds elapsed)]",
			scheduler.PipelineName(), idx+1, PIPELINE_STOP_TIMEOUT_SECONDS)
	}
}

func (scheduler *commonPipelineScheduler) StopPipeline() {
	logger.Debugf("[stopping pipeline %s]", scheduler.PipelineName())

	scheduler.instancesLock.Lock()
	defer scheduler.instancesLock.Unlock()

	for idx, instance := range scheduler.instances {
		scheduler.stopPipelineInstance(idx, instance, false)
	}

	currentParallelism := len(scheduler.instances)

	// no managed instance, re-entry-able function
	scheduler.instances = scheduler.instances[:0]

	logger.Infof("[stopped pipeline %s (parallelism=%d)]", scheduler.PipelineName(), currentParallelism)
}

////

const (
	SCHEDULER_DYNAMIC_SPAWN_MIN_INTERVAL_MS  = 100
	SCHEDULER_DYNAMIC_FAST_SCALE_INTERVAL_MS = 1000
	SCHEDULER_DYNAMIC_FAST_SCALE_RATIO       = 1.5
	SCHEDULER_DYNAMIC_FAST_SCALE_MIN_COUNT   = 5
)

type inputEvent struct {
	getterName  string
	getter      pipelines.SourceInputQueueLengthGetter
	queueLength uint32
}

type dynamicPipelineScheduler struct {
	*commonPipelineScheduler
	ctx            pipelines.PipelineContext
	statistics     *model.PipelineStatistics
	mod            *model.Model
	gettersLock    sync.RWMutex
	getters        map[string]pipelines.SourceInputQueueLengthGetter
	launchChan     chan *inputEvent
	shrinkStop     chan struct{}
	launchTimes    map[string]time.Time
	shrinkTimeLock sync.RWMutex
	shrinkTime     time.Time
}

func newDynamicPipelineScheduler(pipeline *model.Pipeline) *dynamicPipelineScheduler {
	return &dynamicPipelineScheduler{
		commonPipelineScheduler: newCommonPipelineScheduler(pipeline),
		getters:                 make(map[string]pipelines.SourceInputQueueLengthGetter, 1),
		launchChan:              make(chan *inputEvent, 1024), // buffer for trigger() calls before scheduler starts
		shrinkStop:              make(chan struct{}),
		launchTimes:             make(map[string]time.Time, 1),
	}
}

func (scheduler *dynamicPipelineScheduler) SourceInputTrigger() pipelines.SourceInputTrigger {
	return scheduler.trigger
}

func (scheduler *dynamicPipelineScheduler) Start(ctx pipelines.PipelineContext,
	statistics *model.PipelineStatistics, mod *model.Model) {

	if !atomic.CompareAndSwapUint32(&scheduler.started, 0, 1) {
		return // already started
	}

	// book for delay schedule
	scheduler.ctx = ctx
	scheduler.statistics = statistics
	scheduler.mod = mod

	parallelism, _ := scheduler.startPipeline(option.PipelineInitParallelism, ctx, statistics, mod)

	logger.Debugf("[initialized pipeline instance(s) for pipeline %s (total=%d)]",
		scheduler.PipelineName(), parallelism)

	go scheduler.spawn()
	go scheduler.shrink()
}

func (scheduler *dynamicPipelineScheduler) trigger(getterName string, getter pipelines.SourceInputQueueLengthGetter) {
	if atomic.LoadUint32(&scheduler.stopped) == 1 {
		// scheduler is stop
		return
	}

	queueLength := getter()
	if queueLength == 0 {
		// current parallelism is enough
		return
	}

	scheduler.launchChan <- &inputEvent{
		getterName:  getterName,
		getter:      getter,
		queueLength: queueLength,
	}
}

func (scheduler *dynamicPipelineScheduler) spawn() {
	for {
		select {
		case info := <-scheduler.launchChan:
			if info == nil {
				return // channel/scheduler closed, exit
			}

			now := time.Now()
			lastScheduleAt := scheduler.launchTimes[info.getterName]

			if now.Sub(lastScheduleAt).Seconds()*1000 < SCHEDULER_DYNAMIC_SPAWN_MIN_INTERVAL_MS {
				// pipeline instance schedule needs time
				continue
			}

			scheduler.launchTimes[info.getterName] = now

			// book for shrink
			scheduler.gettersLock.Lock()
			scheduler.getters[info.getterName] = info.getter
			scheduler.gettersLock.Unlock()

			scheduler.shrinkTimeLock.RLock()

			if now.Sub(scheduler.shrinkTime).Seconds()*1000 < SCHEDULER_DYNAMIC_FAST_SCALE_INTERVAL_MS {
				// increase is close to decrease, which supposes last shrink reach the real/minimal parallelism
				l := uint32(math.Ceil(float64(info.queueLength) * SCHEDULER_DYNAMIC_FAST_SCALE_RATIO)) // fast scale up
				if l < SCHEDULER_DYNAMIC_FAST_SCALE_MIN_COUNT {
					l = SCHEDULER_DYNAMIC_FAST_SCALE_MIN_COUNT
				}

				if l > info.queueLength { // defense overflow
					info.queueLength = l
				}
			}

			scheduler.shrinkTimeLock.RUnlock()

			parallelism, delta := scheduler.startPipeline(
				info.queueLength, scheduler.ctx, scheduler.statistics, scheduler.mod)

			if delta > 0 {
				logger.Debugf("[spawned pipeline instance(s) for pipeline %s (total=%d, increase=%d)]",
					scheduler.PipelineName(), parallelism, delta)
			}
		}
	}
}

func (scheduler *dynamicPipelineScheduler) shrink() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			scheduler.instancesLock.RLock()

			currentParallelism := uint32(len(scheduler.instances))

			if currentParallelism <= option.PipelineMinParallelism {
				scheduler.instancesLock.RUnlock()
				continue // keep minimal pipeline parallelism
			}

			scheduler.instancesLock.RUnlock()

			scheduler.gettersLock.RLock()

			var queueLength uint32
			for _, getter := range scheduler.getters {
				l := getter()
				if l+queueLength > queueLength { // defense overflow
					queueLength = l + queueLength
				}
			}

			scheduler.gettersLock.RUnlock()

			if queueLength != 0 {
				continue // shrink only
			}

			var instance *pipelineInstance

			scheduler.instancesLock.Lock()

			currentParallelism = uint32(len(scheduler.instances))

			// DCL
			if currentParallelism <= option.PipelineMinParallelism {
				scheduler.instancesLock.Unlock()
				continue // keep minimal pipeline parallelism
			}

			// pop from tail as stack
			idx := int(currentParallelism) - 1
			instance, scheduler.instances = scheduler.instances[idx], scheduler.instances[:idx]

			scheduler.instancesLock.Unlock()

			scheduler.shrinkTimeLock.Lock()
			scheduler.shrinkTime = time.Now()
			scheduler.shrinkTimeLock.Unlock()

			scheduler.stopPipelineInstance(idx, instance, true)

			scheduler.instancesLock.RLock()

			logger.Infof("[shrank a pipeline instance for pipeline %s (total=%d, decrease=%d)]",
				scheduler.PipelineName(), len(scheduler.instances), 1)

			scheduler.instancesLock.RUnlock()
		case <-scheduler.shrinkStop:
			return
		}
	}
}

func (scheduler *dynamicPipelineScheduler) Stop() {
	if !atomic.CompareAndSwapUint32(&scheduler.stopped, 0, 1) {
		return // already stopped
	}

	close(scheduler.launchChan)
	close(scheduler.shrinkStop)

	atomic.StoreUint32(&scheduler.started, 0)
}

////

type staticPipelineScheduler struct {
	*commonPipelineScheduler
}

func newStaticPipelineScheduler(pipeline *model.Pipeline) *staticPipelineScheduler {
	return &staticPipelineScheduler{
		commonPipelineScheduler: newCommonPipelineScheduler(pipeline),
	}
}

func (scheduler *staticPipelineScheduler) SourceInputTrigger() pipelines.SourceInputTrigger {
	return pipelines.NoOpSourceInputTrigger
}

func (scheduler *staticPipelineScheduler) Start(ctx pipelines.PipelineContext,
	statistics *model.PipelineStatistics, mod *model.Model) {

	if !atomic.CompareAndSwapUint32(&scheduler.started, 0, 1) {
		return // already started
	}

	parallelism, _ := scheduler.startPipeline(
		uint32(scheduler.pipeline.Config().Parallelism()), ctx, statistics, mod)

	logger.Debugf("[initialized pipeline instance(s) for pipeline %s (total=%d)]",
		scheduler.PipelineName(), parallelism)
}

func (scheduler *staticPipelineScheduler) Stop() {
	if !atomic.CompareAndSwapUint32(&scheduler.stopped, 0, 1) {
		return // already stopped
	}

	atomic.StoreUint32(&scheduler.started, 0)
}
