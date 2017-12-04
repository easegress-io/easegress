package engine

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"

	"logger"
	"math"
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
	instancesLock    sync.Mutex
	instances        []*pipelineInstance
	started, stopped uint32
}

func newCommonPipelineScheduler(pipeline *model.Pipeline) *commonPipelineScheduler {
	return &commonPipelineScheduler{
		pipeline: pipeline,
		stopped:  2, // initial stop status, Stop() rejects but trigger() handles before scheduler starts
	}
}

func (scheduler *commonPipelineScheduler) PipelineName() string {
	return scheduler.pipeline.Name()
}

func (scheduler *commonPipelineScheduler) startPipeline(parallelism uint32,
	ctx pipelines.PipelineContext, statistics *model.PipelineStatistics, mod *model.Model) uint32 {

	if parallelism == 0 { // defensive
		parallelism = 1
	}

	scheduler.instancesLock.Lock()
	defer scheduler.instancesLock.Unlock()

	currentParallelism := uint32(len(scheduler.instances))

	if atomic.LoadUint32(&scheduler.stopped) == 1 ||
		currentParallelism == ^uint32(0) { // 4294967295
		return currentParallelism // scheduler is stop or reach the cap
	}

	if option.PipelineMaxParallelism > 0 {
		left := option.PipelineMaxParallelism - currentParallelism

		if parallelism > left {
			parallelism = left
		}
	}

	for idx := uint32(0); idx < parallelism; idx++ {
		pipeline, err := scheduler.pipeline.GetInstance(ctx, statistics, mod)
		if err != nil {
			logger.Errorf("[launch pipeline %s-#%d failed: %v]",
				scheduler.PipelineName(), currentParallelism+idx+1, err)

			return currentParallelism
		}

		instance := newPipelineInstance(pipeline)
		scheduler.instances = append(scheduler.instances, instance)
		currentParallelism++

		instance.prepare()
		go instance.run()
	}

	return currentParallelism
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

	logger.Infof("[stopped pipeline %s (parallelism=%d)]", scheduler.PipelineName(), len(scheduler.instances))
}

////

type dynamicPipelineScheduler struct {
	*commonPipelineScheduler
	ctx         pipelines.PipelineContext
	statistics  *model.PipelineStatistics
	mod         *model.Model
	triggers    int64
	gettersLock sync.RWMutex
	getters     map[string]pipelines.SourceInputQueueLengthGetter
	shrinkStop  chan struct{}
}

func newDynamicPipelineScheduler(pipeline *model.Pipeline) *dynamicPipelineScheduler {
	return &dynamicPipelineScheduler{
		commonPipelineScheduler: newCommonPipelineScheduler(pipeline),
		getters:                 make(map[string]pipelines.SourceInputQueueLengthGetter, 1),
		shrinkStop:              make(chan struct{}),
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

	scheduler.startPipeline(option.PipelineInitParallelism, ctx, statistics, mod)

	go scheduler.shrink()

	atomic.StoreUint32(&scheduler.stopped, 0)
}

func (scheduler *dynamicPipelineScheduler) trigger(getterName string, getter pipelines.SourceInputQueueLengthGetter) {
	if atomic.LoadUint32(&scheduler.stopped) == 1 {
		return // scheduler is stop
	}

	queueLength := getter()
	if queueLength == 0 {
		return
	}

	go func() {
		atomic.AddInt64(&scheduler.triggers, 1)
		defer atomic.AddInt64(&scheduler.triggers, -1)

	LOOP:
		for { // wait scheduler start by spin
			switch atomic.LoadUint32(&scheduler.stopped) {
			case 0: // scheduler is start
				break LOOP
			case 1: // scheduler is stop
				return
			default: // 2, initial stop status
				time.Sleep(time.Millisecond) // yield
			}
		}

		queueLength = getter() // update query
		if queueLength == 0 {
			return
		}

		scheduler.gettersLock.Lock()
		scheduler.getters[getterName] = getter
		scheduler.gettersLock.Unlock()

		l := uint32(math.Ceil(float64(queueLength) * 1.2)) // fast scale up, ratio is an option?

		if l > queueLength { // defense overflow
			queueLength = l
		}

		parallelism := scheduler.startPipeline(
			queueLength, scheduler.ctx, scheduler.statistics, scheduler.mod)

		logger.Debugf("[spawned pipeline instance(s) for pipeline %s (total=%d)]",
			scheduler.PipelineName(), parallelism)
	}()
}

func (scheduler *dynamicPipelineScheduler) shrink() {
	for {
		select {
		case <-time.Tick(time.Second):
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

			currentParallelism := uint32(len(scheduler.instances))

			if currentParallelism <= option.PipelineMinParallelism {
				scheduler.instancesLock.Unlock()
				continue // keep minimal pipeline parallelism
			}

			// pop from tail as stack
			idx := int(currentParallelism) - 1
			instance, scheduler.instances = scheduler.instances[idx], scheduler.instances[:idx]

			scheduler.instancesLock.Unlock()

			scheduler.stopPipelineInstance(idx, instance, true)

			logger.Debugf("[shrank a pipeline instance for pipeline %s (total=%d)]",
				scheduler.PipelineName(), idx)
		case <-scheduler.shrinkStop:
			return
		}
	}
}

func (scheduler *dynamicPipelineScheduler) Stop() {
	if !atomic.CompareAndSwapUint32(&scheduler.stopped, 0, 1) {
		return // already stopped or under initial stop status
	}

	// spin to wait trigger goroutine exits
	for atomic.LoadInt64(&scheduler.triggers) > 0 {
		time.Sleep(time.Millisecond) // yield
	}

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

	scheduler.startPipeline(uint32(scheduler.pipeline.Config().Parallelism()), ctx, statistics, mod)

	atomic.StoreUint32(&scheduler.stopped, 0)
}

func (scheduler *staticPipelineScheduler) Stop() {
	if !atomic.CompareAndSwapUint32(&scheduler.stopped, 0, 1) {
		return // already stopped
	}

	atomic.StoreUint32(&scheduler.started, 0)
}
