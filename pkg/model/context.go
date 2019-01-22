package model

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/store"

	"github.com/megaease/easegateway/pkg/pipelines"
)

//
// Pipeline context
//

type pipelineContextClosingCallback func(ctx pipelines.PipelineContext)

type bucketItem struct {
	bucket     *pipelineContextDataBucket
	autoDelete bool
}

type pipelineContext struct {
	pipeName             string
	plugNames            []string
	parallelismCount     uint16
	statistics           pipelines.PipelineStatistics
	mod                  *Model
	bucketLock           sync.RWMutex // dedicated lock provides better performance
	buckets              map[string]*bucketItem
	requestChanLock      sync.Mutex
	requestChan          chan *pipelines.DownstreamRequest
	trigger              pipelines.SourceInputTrigger
	closingCallbacksLock sync.RWMutex
	closingCallbacks     *common.NamedCallbackSet
	pluginDeleteChan     chan *Plugin
}

func newPipelineContext(spec *store.PipelineSpec, statistics pipelines.PipelineStatistics,
	m *Model, trigger pipelines.SourceInputTrigger) *pipelineContext {

	if trigger == nil { // defensive
		trigger = pipelines.NoOpSourceInputTrigger
	}

	c := &pipelineContext{
		pipeName:         spec.Name,
		plugNames:        spec.Config.Plugins,
		parallelismCount: spec.Config.ParallelismCount,
		statistics:       statistics,
		mod:              m,
		buckets:          make(map[string]*bucketItem),
		requestChan:      make(chan *pipelines.DownstreamRequest, spec.Config.CrossPipelineRequestBacklogLength),
		trigger:          trigger,
		closingCallbacks: common.NewNamedCallbackSet(),
		pluginDeleteChan: make(chan *Plugin, 1024),
	}

	go c.deletePipelineContextDataBucketWhenPluginDeleted()

	logger.Infof("pipeline %s context at %p is created", spec.Name, c)

	return c
}

func (pc *pipelineContext) PipelineName() string {
	return pc.pipeName
}

func (pc *pipelineContext) PluginNames() []string {
	return pc.plugNames
}

func (pc *pipelineContext) Statistics() pipelines.PipelineStatistics {
	return pc.statistics
}

func (pc *pipelineContext) DataBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	deleteWhenPluginDeleted := false

	if len(strings.TrimSpace(pluginInstanceId)) == 0 {
		pluginInstanceId = pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE
		deleteWhenPluginDeleted = true
	} else if pluginInstanceId == pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE {
		deleteWhenPluginDeleted = true
	}

	bucketKey := fmt.Sprintf("%s-%s", pluginName, pluginInstanceId)

	pc.bucketLock.RLock()
	item, exists := pc.buckets[bucketKey]
	if exists {
		pc.bucketLock.RUnlock()
		return item.bucket
	}
	pc.bucketLock.RUnlock()

	pc.bucketLock.Lock()

	item, exists = pc.buckets[bucketKey]
	if exists { // DCL
		pc.bucketLock.Unlock()
		return item.bucket
	}

	bucket := newPipelineContextDataBucket(pc.pipeName)

	pc.buckets[bucketKey] = &bucketItem{
		bucket:     bucket,
		autoDelete: deleteWhenPluginDeleted,
	}

	pc.bucketLock.Unlock()

	return bucket
}

func (pc *pipelineContext) DeleteBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	if len(strings.TrimSpace(pluginInstanceId)) == 0 {
		pluginInstanceId = pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE
	}

	var oriBucket *bucketItem
	updatedBucket := make(map[string]*bucketItem)

	bucketKey := fmt.Sprintf("%s-%s", pluginName, pluginInstanceId)

	pc.bucketLock.Lock()
	defer pc.bucketLock.Unlock()

	for key, bucketItem := range pc.buckets {
		if key == bucketKey {
			oriBucket = bucketItem
		} else {
			updatedBucket[key] = bucketItem
		}
	}

	pc.buckets = updatedBucket

	if oriBucket == nil {
		return nil
	}

	oriBucket.bucket.close()

	return oriBucket.bucket
}

func (pc *pipelineContext) CommitCrossPipelineRequest(
	request *pipelines.DownstreamRequest, cancel <-chan struct{}) error {

	if request == nil {
		return fmt.Errorf("request is nil")
	}

	if request.UpstreamPipelineName() == pc.pipeName {
		if pc.requestChan == nil {
			return fmt.Errorf("request processing queue of pipeline %s is closed",
				request.UpstreamPipelineName())
		}

		return func() (err error) {
			defer func() {
				// to prevent send on closed channel due to
				// Close() of the pipeline context can be called concurrently
				e := recover()
				if e != nil {
					err = fmt.Errorf("request processing queue of pipeline %s is closed",
						request.UpstreamPipelineName())
				}
			}()

			select {
			case pc.requestChan <- request:
				pc.TriggerSourceInput("crossPipelineRequestQueueLengthGetter", pc.getCrossPipelineRequestQueueLength)
				err = nil
			case <-cancel:
				err = fmt.Errorf("request is canclled")
			}

			return
		}()
	} else { // cross to the correct pipeline context
		contexts := pc.mod.pipelineContexts()
		ctx := contexts[request.UpstreamPipelineName()]
		if ctx == nil {
			return fmt.Errorf("the context of pipeline %s not found",
				request.UpstreamPipelineName())
		}

		return ctx.CommitCrossPipelineRequest(request, cancel)
	}
}

func (pc *pipelineContext) getCrossPipelineRequestQueueLength() uint32 {
	return uint32(len(pc.requestChan))
}

func (pc *pipelineContext) ClaimCrossPipelineRequest(cancel <-chan struct{}) *pipelines.DownstreamRequest {
	// to use recover() way instead of lock pc.requestChanLock since
	// Close() of the pipeline context should be able to support concurrent call with this function
	// (this function can be blocked on channel receiving)
	return func() (ret *pipelines.DownstreamRequest) {
		defer func() {
			// to prevent receive on nil channel due to
			// Close() of the pipeline context can be called concurrently
			e := recover()
			if e != nil {
				ret = nil
			}
		}()

		select {
		case ret = <-pc.requestChan:
			// Nothing to do
		case <-cancel:
			ret = nil
		}
		return
	}()
}

func (pc *pipelineContext) CrossPipelineWIPRequestsCount(upstreamPipelineName string) int {
	if upstreamPipelineName == pc.pipeName {
		return len(pc.requestChan)
	} else { // cross to the correct pipeline context
		contexts := pc.mod.pipelineContexts()
		ctx := contexts[upstreamPipelineName]
		if ctx == nil {
			logger.Warnf("the context of upstream pipeline %s not found", upstreamPipelineName)
			return 0
		}

		return ctx.CrossPipelineWIPRequestsCount(upstreamPipelineName)
	}
}

func (pc *pipelineContext) TriggerSourceInput(getterName string, getter pipelines.SourceInputQueueLengthGetter) {
	pc.trigger(getterName, getter)
}

func (pc *pipelineContext) Close() {
	// to guarantee call close() on channel only once
	pc.requestChanLock.Lock()
	defer pc.requestChanLock.Unlock()

	if pc.requestChan != nil {
		// defensive programming on reentry
		close(pc.requestChan)
		pc.requestChan = nil // to make len() returns 0 in CrossPipelineWIPRequestsCount()
	}

	tmp := pc.closingCallbacks.CopyCallbacks()

	for _, callback := range tmp {
		callback.Callback().(pipelineContextClosingCallback)(pc)
	}

	for _, bucketItem := range pc.buckets {
		bucketItem.bucket.close()
	}

	logger.Infof("pipeline %s context at %p is closed", pc.pipeName, pc)
}

func (pc *pipelineContext) deletePipelineContextDataBucketWhenPluginDeleted() {
	for {
		<-pc.pluginDeleteChan
		bucketInUsed := func(bucketKey string) bool {
			// defensive the case plugin instance closes after it was deleted from model
			for _, pluginName := range pc.PluginNames() {
				if strings.HasPrefix(bucketKey, fmt.Sprintf("%s-", pluginName)) {
					return true
				}
			}

			return false
		}

		pc.bucketLock.Lock()

		updatedBucket := make(map[string]*bucketItem)
		for key, bucketItem := range pc.buckets {
			if bucketInUsed(key) || !bucketItem.autoDelete {
				updatedBucket[key] = bucketItem
			} else {
				bucketItem.bucket.close()
			}
		}
		pc.buckets = updatedBucket

		pc.bucketLock.Unlock()
	}
}

func (pc *pipelineContext) addClosingCallback(name string, callback pipelineContextClosingCallback, priority string) {
	pc.closingCallbacksLock.Lock()
	pc.closingCallbacks = common.AddCallback(pc.closingCallbacks, name, callback, priority)
	pc.closingCallbacksLock.Unlock()
}

func (pc *pipelineContext) deleteClosingCallback(name string) {
	pc.closingCallbacksLock.Lock()
	pc.closingCallbacks = common.DeleteCallback(pc.closingCallbacks, name)
	pc.closingCallbacksLock.Unlock()
}

//
// Pipeline context data bucket
//

type pipelineContextDataBucket struct {
	sync.RWMutex
	pipelineName string
	data         map[interface{}]interface{}
}

func newPipelineContextDataBucket(pipelineName string) *pipelineContextDataBucket {
	return &pipelineContextDataBucket{
		pipelineName: pipelineName,
		data:         make(map[interface{}]interface{}),
	}
}

func (b *pipelineContextDataBucket) BindData(key, value interface{}) (interface{}, error) {
	b.Lock()
	defer b.Unlock()
	return b.bindData(key, value)
}

func (b *pipelineContextDataBucket) bindData(key, value interface{}) (interface{}, error) {
	if key == nil {
		return nil, fmt.Errorf("key is nil")
	}

	if !reflect.TypeOf(key).Comparable() {
		return nil, fmt.Errorf("key is not comparable")
	}

	oriData := b.data[key]
	b.data[key] = value
	return oriData, nil
}

func (b *pipelineContextDataBucket) QueryData(key interface{}) interface{} {
	b.RLock()
	defer b.RUnlock()
	return b.data[key]
}

func (b *pipelineContextDataBucket) QueryDataWithBindDefault(key interface{},
	defaultValueFunc pipelines.DefaultValueFunc) (interface{}, error) {

	b.RLock()

	value, exists := b.data[key]
	if exists {
		b.RUnlock()
		return value, nil
	}

	b.RUnlock()

	b.Lock()
	defer b.Unlock()

	value, exists = b.data[key]
	if exists { // DCL
		return value, nil
	} else {
		value = defaultValueFunc()
		_, err := b.bindData(key, value)
		if err != nil {
			return nil, err
		}
	}

	return value, nil
}

func (b *pipelineContextDataBucket) UnbindData(key interface{}) interface{} {
	b.Lock()
	defer b.Unlock()

	oriData := b.data[key]
	delete(b.data, key)
	return oriData
}

func (b *pipelineContextDataBucket) close() {
	b.RLock()
	defer b.RUnlock()

	for _, value := range b.data {
		closer, ok := value.(io.Closer)
		if ok {
			err := closer.Close()
			if err != nil {
				logger.Warnf("close data in the data bucket of the pipeline %s failed, ignored: %v",
					b.pipelineName, err)
			}
		}
	}
}
