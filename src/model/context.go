package model

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/hexdecteam/easegateway-types/pipelines"

	"common"
	"logger"
	pipelines_gw "pipelines"
)

//
// Pipeline context
//

type bucketItem struct {
	bucket     pipelines.PipelineContextDataBucket
	autoDelete bool
}

type pipelineContext struct {
	pipeName         string
	plugNames        []string
	parallelismCount uint16
	statistics       pipelines.PipelineStatistics
	mod              *Model
	bucketLock       sync.RWMutex // dedicated lock provides better performance
	buckets          map[string]*bucketItem
	bookLock         sync.RWMutex
	preparationBook  map[string]interface{} // hash here for O(1) time on query
	requestChanLock  sync.Mutex
	requestChan      chan *pipelines.DownstreamRequest
}

func NewPipelineContext(conf pipelines_gw.Config,
	statistics pipelines.PipelineStatistics, m *Model) *pipelineContext {

	return &pipelineContext{
		pipeName:         conf.PipelineName(),
		plugNames:        conf.PluginNames(),
		parallelismCount: conf.Parallelism(),
		statistics:       statistics,
		mod:              m,
		buckets:          make(map[string]*bucketItem),
		preparationBook:  make(map[string]interface{}),
		requestChan:      make(chan *pipelines.DownstreamRequest, conf.CrossPipelineRequestBacklog()),
	}
}

func (pc *pipelineContext) PipelineName() string {
	return pc.pipeName
}

func (pc *pipelineContext) PluginNames() []string {
	return pc.plugNames
}

func (pc *pipelineContext) Parallelism() uint16 {
	return pc.parallelismCount
}

func (pc *pipelineContext) Statistics() pipelines.PipelineStatistics {
	return pc.statistics
}

func (pc *pipelineContext) DataBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	deleteWhenPluginUpdatedOrDeleted := false

	if len(strings.TrimSpace(pluginInstanceId)) == 0 {
		pluginInstanceId = pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE
	} else if pluginInstanceId != pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE {
		deleteWhenPluginUpdatedOrDeleted = true
	}

	pc.bucketLock.Lock()

	bucketKey := fmt.Sprintf("%s-%s", pluginName, pluginInstanceId)

	item, exists := pc.buckets[bucketKey]
	if exists {
		defer pc.bucketLock.Unlock()
		return item.bucket
	}

	bucket := newPipelineContextDataBucket()

	pc.buckets[bucketKey] = &bucketItem{
		bucket:     bucket,
		autoDelete: deleteWhenPluginUpdatedOrDeleted,
	}

	defer pc.bucketLock.Unlock()

	if deleteWhenPluginUpdatedOrDeleted {
		pc.mod.AddPluginDeletedCallback("deletePipelineContextDataBucketWhenPluginDeleted",
			pc.deletePipelineContextDataBucketWhenPluginDeleted, false)
		pc.mod.AddPluginUpdatedCallback("deletePipelineContextDataBucketWhenPluginUpdated",
			pc.deletePipelineContextDataBucketWhenPluginUpdated, false)
	}

	return bucket
}

func (pc *pipelineContext) DeleteBucket(pluginName, pluginInstanceId string) pipelines.PipelineContextDataBucket {
	if len(strings.TrimSpace(pluginInstanceId)) == 0 {
		pluginInstanceId = pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE
	}

	var oriBucket pipelines.PipelineContextDataBucket
	updatedBucket := make(map[string]*bucketItem)

	pc.bucketLock.Lock()
	defer pc.bucketLock.Unlock()

	for bucketKey, bucketItem := range pc.buckets {
		if bucketKey == fmt.Sprintf("%s-%s", pluginName, pluginInstanceId) {
			oriBucket = bucketItem.bucket
		} else {
			updatedBucket[bucketKey] = bucketItem
		}
	}

	pc.buckets = updatedBucket

	return oriBucket
}

func (pc *pipelineContext) PreparePlugin(pluginName string, fun pipelines.PluginPreparationFunc) {
	pc.bookLock.RLock()
	_, exists := pc.preparationBook[pluginName]
	if exists {
		pc.bookLock.RUnlock()
		return
	}
	pc.bookLock.RUnlock()

	pc.bookLock.Lock()
	defer pc.bookLock.Unlock()

	// DCL
	_, exists = pc.preparationBook[pluginName]
	if exists {
		return
	}

	logger.Debugf("[prepare plugin %s for pipeline %s]", pluginName, pc.pipeName)
	fun()
	logger.Debugf("[plugin %s prepared for pipeline %s]", pluginName, pc.pipeName)

	pc.mod.AddPluginDeletedCallback("deletePluginPreparationBookWhenPluginUpdatedOrDeleted",
		pc.deletePluginPreparationBookWhenPluginUpdatedOrDeleted, false)
	pc.mod.AddPluginUpdatedCallback("deletePluginPreparationBookWhenPluginUpdatedOrDeleted",
		pc.deletePluginPreparationBookWhenPluginUpdatedOrDeleted, false)

	pc.preparationBook[pluginName] = nil
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
				err = nil
			case <-cancel:
				err = fmt.Errorf("request is canclled")
			}

			return
		}()
	} else { // cross to the correct pipeline context
		ctx := pc.mod.GetPipelineContext(request.UpstreamPipelineName())
		if ctx == nil {
			return fmt.Errorf("the context of pipeline %s not found",
				request.UpstreamPipelineName())
		}

		return ctx.CommitCrossPipelineRequest(request, cancel)
	}
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
		ctx := pc.mod.GetPipelineContext(upstreamPipelineName)
		if ctx == nil {
			logger.Warnf("[the context of upstream pipeline %s not found]", upstreamPipelineName)
			return 0
		}

		return ctx.CrossPipelineWIPRequestsCount(upstreamPipelineName)
	}
}

func (pc *pipelineContext) Close() {
	pc.mod.DeletePluginDeletedCallback("deletePipelineContextDataBucketWhenPluginDeleted")
	pc.mod.DeletePluginUpdatedCallback("deletePipelineContextDataBucketWhenPluginUpdated")

	pc.mod.DeletePluginDeletedCallback("deletePluginPreparationBookWhenPluginUpdatedOrDeleted")
	pc.mod.DeletePluginUpdatedCallback("deletePluginPreparationBookWhenPluginUpdatedOrDeleted")

	// to guarantee call close() on channel only once
	pc.requestChanLock.Lock()
	defer pc.requestChanLock.Unlock()

	if pc.requestChan != nil {
		// defensive programming on reentry
		close(pc.requestChan)
		pc.requestChan = nil // to make len() returns 0 in CrossPipelineWIPRequestsCount()
	}
}

func (pc *pipelineContext) deletePluginPreparationBookWhenPluginUpdatedOrDeleted(plugin *Plugin) {
	if !common.StrInSlice(plugin.Name(), pc.plugNames) {
		return
	}

	pc.bookLock.Lock()
	defer pc.bookLock.Unlock()
	delete(pc.preparationBook, plugin.Name())
}

func (pc *pipelineContext) deletePipelineContextDataBucketWhenPluginDeleted(_ *Plugin) {
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
	defer pc.bucketLock.Unlock()

	updatedBucket := make(map[string]*bucketItem)

	for bucketKey, bucketItem := range pc.buckets {
		if bucketInUsed(bucketKey) || !bucketItem.autoDelete {
			updatedBucket[bucketKey] = bucketItem
		}
	}

	pc.buckets = updatedBucket
}

func (pc *pipelineContext) deletePipelineContextDataBucketWhenPluginUpdated(p *Plugin) {
	updatedBucket := make(map[string]*bucketItem)

	pc.bucketLock.Lock()
	defer pc.bucketLock.Unlock()

	for bucketKey, bucketItem := range pc.buckets {
		if !strings.HasPrefix(bucketKey, fmt.Sprintf("%s-", p.Name())) || !bucketItem.autoDelete {
			updatedBucket[bucketKey] = bucketItem
		}
	}

	pc.buckets = updatedBucket
}

//
// Pipeline context data bucket
//

type pipelineContextDataBucket struct {
	sync.RWMutex
	data map[interface{}]interface{}
}

func newPipelineContextDataBucket() *pipelineContextDataBucket {
	return &pipelineContextDataBucket{
		data: make(map[interface{}]interface{}),
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

	b.Lock()
	defer b.Unlock()

	var value interface{}

	_value, exists := b.data[key]
	if !exists {
		value = defaultValueFunc()
		_, err := b.bindData(key, value)
		if err != nil {
			return nil, err
		}
	} else {
		value = _value
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
