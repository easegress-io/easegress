package model

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/store"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"

	"github.com/megaease/easegateway/pkg/pipelines"
)

type pipelineInstance struct {
	instance *Pipeline
	stop     chan struct{}
	stopped  chan struct{}
	done     chan struct{}
}

func newPipelineInstance(instance *Pipeline) *pipelineInstance {
	return &pipelineInstance{
		instance: instance,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
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
					"pipeline %s runs error and exits exceptionally: %v",
					pi.instance.Name(), err)
				break loop
			}
		}
	}

	<-pi.stopped
	pi.instance.Close()
	close(pi.done)
}

func (pi *pipelineInstance) terminate(scheduled bool) chan struct{} {
	close(pi.stop)
	go func() { // Stop() blocks until Run() exits
		pi.instance.Stop(scheduled)
		close(pi.stopped)
	}()
	return pi.done
}

////

type PipelineScheduler interface {
	PipelineName() string
	PipelineContext() *pipelineContext
	SourceInputTrigger() pipelines.SourceInputTrigger
	PipelineInstances() []*pipelineInstance
	Start()
	Stop()
	StopPipeline()
}

////

const PIPELINE_STOP_TIMEOUT_SECONDS = 30

type commonPipelineScheduler struct {
	spec             *store.PipelineSpec
	mod              *Model
	stat             *PipelineStatistics
	ctx              *pipelineContext
	instancesLock    sync.RWMutex
	instances        []*pipelineInstance
	started, stopped uint32
}

func newCommonPipelineScheduler(spec *store.PipelineSpec, mod *Model,
	trigger pipelines.SourceInputTrigger) *commonPipelineScheduler {
	scheduler := &commonPipelineScheduler{
		spec: spec,
		mod:  mod,
		stat: NewPipelineStatistics(spec, mod),
	}
	scheduler.ctx = newPipelineContext(spec, scheduler.stat, mod, trigger)

	return scheduler
}

func (scheduler *commonPipelineScheduler) PipelineName() string {
	return scheduler.spec.Name
}

func (scheduler *commonPipelineScheduler) PipelineContext() *pipelineContext {
	return scheduler.ctx
}

func (scheduler *commonPipelineScheduler) PipelineInstances() []*pipelineInstance {
	scheduler.instancesLock.RLock()
	defer scheduler.instancesLock.RUnlock()
	return scheduler.instances
}

func (scheduler *commonPipelineScheduler) startPipeline(parallelism uint32) (uint32, uint32) {

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

	left := option.Global.PipelineMaxParallelism - currentParallelism
	if parallelism > left {
		parallelism = left
	}

	idx := uint32(0)
	for idx < parallelism {
		pipeline, err := GetPipelineInstance(scheduler.spec, scheduler.ctx,
			scheduler.stat, scheduler.mod)
		if err != nil {
			logger.Errorf("launch pipeline %s-#%d failed: %v",
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
		logger.Warnf("stopped pipeline %s instance #%d timeout (%d seconds elapsed)",
			scheduler.PipelineName(), idx+1, PIPELINE_STOP_TIMEOUT_SECONDS)
	}
}

func (scheduler *commonPipelineScheduler) StopPipeline() {
	logger.Debugf("stopping pipeline %s", scheduler.PipelineName())

	scheduler.instancesLock.Lock()
	defer scheduler.instancesLock.Unlock()

	for idx, instance := range scheduler.instances {
		scheduler.stopPipelineInstance(idx, instance, false)
	}

	currentParallelism := len(scheduler.instances)

	// no managed instance, re-entry-able function
	scheduler.instances = scheduler.instances[:0]

	logger.Infof("stopped pipeline %s (parallelism=%d)", scheduler.PipelineName(), currentParallelism)
}

func (scheduler *commonPipelineScheduler) Close() {
	scheduler.ctx.Close()
	scheduler.stat.Close()
}

const (
	SCHEDULER_DYNAMIC_SPAWN_MIN_INTERVAL_MS  = 500
	SCHEDULER_DYNAMIC_SPAWN_MAX_IN_EACH      = 500
	SCHEDULER_DYNAMIC_FAST_SCALE_INTERVAL_MS = 1000
	SCHEDULER_DYNAMIC_FAST_SCALE_RATIO       = 1.2
	SCHEDULER_DYNAMIC_FAST_SCALE_MIN_COUNT   = 5
	SCHEDULER_DYNAMIC_SHRINK_MIN_DELAY_MS    = 500
)

type inputEvent struct {
	getterName  string
	getter      pipelines.SourceInputQueueLengthGetter
	queueLength uint32
}

type dynamicPipelineScheduler struct {
	*commonPipelineScheduler
	gettersLock             sync.RWMutex
	getters                 map[string]pipelines.SourceInputQueueLengthGetter
	launchChan              chan *inputEvent
	spawnStop, spawnDone    chan struct{}
	shrinkStop              chan struct{}
	sourceLastScheduleTimes map[string]time.Time
	launchTimeLock          sync.RWMutex
	launchTime              time.Time
	shrinkTimeLock          sync.RWMutex
	shrinkTime              time.Time
}

func newDynamicPipelineScheduler(spec *store.PipelineSpec, mod *Model) *dynamicPipelineScheduler {
	scheduler := &dynamicPipelineScheduler{
		getters:                 make(map[string]pipelines.SourceInputQueueLengthGetter, 1),
		launchChan:              make(chan *inputEvent, 128), // buffer for trigger() calls before scheduler starts
		spawnStop:               make(chan struct{}),
		spawnDone:               make(chan struct{}),
		shrinkStop:              make(chan struct{}),
		sourceLastScheduleTimes: make(map[string]time.Time, 1),
	}
	scheduler.commonPipelineScheduler = newCommonPipelineScheduler(spec, mod, scheduler.SourceInputTrigger())

	return scheduler
}

func (scheduler *dynamicPipelineScheduler) SourceInputTrigger() pipelines.SourceInputTrigger {
	return scheduler.trigger
}

func (scheduler *dynamicPipelineScheduler) Start() {
	if !atomic.CompareAndSwapUint32(&scheduler.started, 0, 1) {
		return // already started
	}

	parallelism, _ := scheduler.startPipeline(option.Global.PipelineInitParallelism)

	logger.Debugf("initialized pipeline instance(s) for pipeline %s (total=%d)",
		scheduler.PipelineName(), parallelism)

	go scheduler.launch()
	go scheduler.spawn()
	go scheduler.shrink()
}

func (scheduler *dynamicPipelineScheduler) trigger(getterName string, getter pipelines.SourceInputQueueLengthGetter) {
	queueLength := getter()
	if queueLength == 0 {
		// current parallelism is enough
		return
	}

	if atomic.LoadUint32(&scheduler.stopped) == 1 {
		// scheduler is stop
		return
	}

	event := &inputEvent{
		getterName:  getterName,
		getter:      getter,
		queueLength: queueLength,
	}

	select {
	case scheduler.launchChan <- event:
	default: // skip if busy, spawn() routine will redress
	}
}

func (scheduler *dynamicPipelineScheduler) launch() {
	for {
		select {
		case info := <-scheduler.launchChan:
			if info == nil {
				return // channel/scheduler closed, exit
			}

			now := common.Now()

			if info.getterName != "" && info.getter != nil { // calls from trigger()
				lastScheduleAt := scheduler.sourceLastScheduleTimes[info.getterName]

				if now.Sub(lastScheduleAt).Seconds()*1000 < SCHEDULER_DYNAMIC_SPAWN_MIN_INTERVAL_MS {
					// pipeline instance schedule needs time
					continue
				}

				scheduler.sourceLastScheduleTimes[info.getterName] = now

				// book for spawn and shrink
				scheduler.gettersLock.Lock()
				scheduler.getters[info.getterName] = info.getter
				scheduler.gettersLock.Unlock()
			} else { // calls from spawn()
				for getterName := range scheduler.sourceLastScheduleTimes {
					scheduler.sourceLastScheduleTimes[getterName] = now
				}
			}

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

			if info.queueLength > SCHEDULER_DYNAMIC_SPAWN_MAX_IN_EACH {
				info.queueLength = SCHEDULER_DYNAMIC_SPAWN_MAX_IN_EACH
			}

			scheduler.shrinkTimeLock.RUnlock()

			parallelism, delta := scheduler.startPipeline(info.queueLength)

			if delta > 0 {
				scheduler.launchTimeLock.Lock()
				scheduler.launchTime = common.Now()
				scheduler.launchTimeLock.Unlock()

				logger.Debugf("spawned pipeline instance(s) for pipeline %s (total=%d, increase=%d)",
					scheduler.PipelineName(), parallelism, delta)
			}
		}
	}
}

func (scheduler *dynamicPipelineScheduler) spawn() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer close(scheduler.spawnDone)

	for {
		select {
		case <-ticker.C:
			scheduler.instancesLock.RLock()

			currentParallelism := uint32(len(scheduler.instances))

			if currentParallelism == option.Global.PipelineMaxParallelism {
				scheduler.instancesLock.RUnlock()
				continue // less than the cap of pipeline parallelism
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

			if queueLength == 0 {
				// current parallelism is enough
				continue // spawn only
			}

			scheduler.launchChan <- &inputEvent{
				queueLength: queueLength,
			} // without getterName and getter
		case <-scheduler.spawnStop:
			return
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

			if currentParallelism <= option.Global.PipelineMinParallelism {
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
			if currentParallelism <= option.Global.PipelineMinParallelism {
				scheduler.instancesLock.Unlock()
				continue // keep minimal pipeline parallelism
			}

			now := common.Now()

			scheduler.launchTimeLock.RLock()

			if now.Sub(scheduler.launchTime).Seconds()*1000 < SCHEDULER_DYNAMIC_SHRINK_MIN_DELAY_MS {
				// just launched instance, need to wait it runs
				scheduler.instancesLock.Unlock()
				scheduler.launchTimeLock.RUnlock()
				continue
			}

			scheduler.launchTimeLock.RUnlock()

			// pop from tail as stack
			idx := int(currentParallelism) - 1
			instance, scheduler.instances = scheduler.instances[idx], scheduler.instances[:idx]

			scheduler.instancesLock.Unlock()

			scheduler.shrinkTimeLock.Lock()
			scheduler.shrinkTime = now
			scheduler.shrinkTimeLock.Unlock()

			scheduler.stopPipelineInstance(idx, instance, true)

			scheduler.instancesLock.RLock()

			logger.Infof("shrank a pipeline instance for pipeline %s (total=%d, decrease=%d)",
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

	close(scheduler.spawnStop)
	close(scheduler.shrinkStop)

	<-scheduler.spawnDone

	close(scheduler.launchChan)

	atomic.StoreUint32(&scheduler.started, 0)

	scheduler.commonPipelineScheduler.Close()
}

type staticPipelineScheduler struct {
	*commonPipelineScheduler
}

func CreatePipelineScheduler(spec *store.PipelineSpec, mod *Model) PipelineScheduler {
	var scheduler PipelineScheduler
	if spec.Config.Parallelism() == 0 { // dynamic mode
		scheduler = newDynamicPipelineScheduler(spec, mod)
	} else { // pre-alloc mode
		scheduler = newStaticPipelineScheduler(spec, mod)
	}
	return scheduler
}

func newStaticPipelineScheduler(spec *store.PipelineSpec, mod *Model) *staticPipelineScheduler {
	scheduler := &staticPipelineScheduler{}
	scheduler.commonPipelineScheduler = newCommonPipelineScheduler(spec, mod, scheduler.SourceInputTrigger())
	return scheduler
}

func (scheduler *staticPipelineScheduler) SourceInputTrigger() pipelines.SourceInputTrigger {
	return pipelines.NoOpSourceInputTrigger
}

func (scheduler *staticPipelineScheduler) Start() {
	if !atomic.CompareAndSwapUint32(&scheduler.started, 0, 1) {
		return // already started
	}

	parallelism, _ := scheduler.startPipeline(uint32(scheduler.spec.Config.ParallelismCount))

	logger.Debugf("initialized pipeline instance(s) for pipeline %s (total=%d)",
		scheduler.PipelineName(), parallelism)
}

func (scheduler *staticPipelineScheduler) Stop() {
	if !atomic.CompareAndSwapUint32(&scheduler.stopped, 0, 1) {
		return // already stopped
	}

	atomic.StoreUint32(&scheduler.started, 0)

	scheduler.commonPipelineScheduler.Close()
}
