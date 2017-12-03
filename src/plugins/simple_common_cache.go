package plugins

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"logger"
	"option"
	"common"
)

type simpleCommonCacheConfig struct {
	common.PluginCommonConfig
	HitKeys     []string `json:"hit_keys"`
	CacheKey    string   `json:"cache_key"`
	TTLSec      uint32   `json:"ttl_sec"` // up to 4294967295, zero means infinite time to live
	FinishIfHit bool     `json:"finish_if_hit"`
}

func simpleCommonCacheConfigConstructor() plugins.Config {
	return &simpleCommonCacheConfig{
		TTLSec:      600, // 10 minutes
		FinishIfHit: true,
	}
}

func (c *simpleCommonCacheConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if len(c.HitKeys) == 0 {
		return fmt.Errorf("hit keys are empty")
	}

	for _, key := range c.HitKeys {
		if len(strings.TrimSpace(key)) == 0 {
			return fmt.Errorf("invalid hit key")
		}
	}

	c.CacheKey = strings.TrimSpace(c.CacheKey)

	if len(c.CacheKey) == 0 {
		return fmt.Errorf("cache key requied")
	}

	if c.TTLSec == 0 {
		logger.Warnf("[ZERO TTL has been applied, no cached item could be released!]")
	}

	return nil
}

type simpleCommonCache struct {
	conf *simpleCommonCacheConfig
}

func simpleCommonCacheConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*simpleCommonCacheConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, false, fmt.Errorf(
			"config type want *simpleCommonCacheConfig got %T", conf)
	}

	return &simpleCommonCache{
		conf: c,
	}, plugins.ProcessPlugin, false, nil
}

func (c *simpleCommonCache) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (c *simpleCommonCache) Run(ctx pipelines.PipelineContext, t task.Task) error {
	c.saveToCache(ctx, t)
	t = c.loadFromCache(ctx, t)
	return nil
}

func (c *simpleCommonCache) Name() string {
	return c.conf.PluginName()
}

func (c *simpleCommonCache) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (c *simpleCommonCache) Close() {
	// Nothing to do.
}

func (c *simpleCommonCache) cacheKey(t task.Task) uint32 {
	h := fnv.New32a()

	for _, key := range c.conf.HitKeys {
		v := t.Value(key)
		if v == nil {
			return 0
		}

		h.Write([]byte(key))
		h.Write([]byte(task.ToString(v, option.PluginIODataFormatLengthLimit)))
	}

	return h.Sum32()
}

func (c *simpleCommonCache) releaseCacheTimer(ctx pipelines.PipelineContext, cacheKey uint32) *time.Timer {
	var ret *time.Timer

	if c.conf.TTLSec > 0 {
		ret = time.AfterFunc(time.Duration(c.conf.TTLSec)*time.Second, func() {
			state, err := getSimpleCommonCacheState(ctx, c.Name())
			if err != nil {
				return
			}

			state.Lock()
			delete(state.cache, cacheKey)
			state.Unlock()
		})
	}

	return ret
}

func (c *simpleCommonCache) saveToCache(ctx pipelines.PipelineContext, t task.Task) {
	cacheItem := func(t1 task.Task, _ task.TaskStatus) {
		// cache successful result only
		if t1.Error() != nil {
			return
		}

		// to prevent loop caching
		cacheFlag := t1.Value(fmt.Sprintf("%s_CACHED", c.Name()))
		if cacheFlag != nil {
			return
		}

		data := t1.Value(c.conf.CacheKey)
		if data == nil {
			return
		}

		cacheKey := c.cacheKey(t)
		if cacheKey == 0 { // missing hit key
			return
		}

		state, err := getSimpleCommonCacheState(ctx, c.Name())
		if err != nil {
			return
		}

		state.Lock()
		defer state.Unlock()

		cacheItem, exists := state.cache[cacheKey]
		if !exists {
			cacheItem = &simpleCommonCacheItem{
				name:  c.conf.CacheKey,
				data:  data,
				timer: c.releaseCacheTimer(ctx, cacheKey),
			}
			state.cache[cacheKey] = cacheItem
		} else {
			cacheItem.name = c.conf.CacheKey
			cacheItem.data = data
		}
	}

	t.AddFinishedCallback(fmt.Sprintf("%s-cacheItem", c.Name()), cacheItem)
}

func (c *simpleCommonCache) loadFromCache(ctx pipelines.PipelineContext, t task.Task) task.Task {
	state, err := getSimpleCommonCacheState(ctx, c.Name())
	if err != nil {
		return t
	}

	state.Lock()
	defer state.Unlock()

	cacheKey := c.cacheKey(t)

	cacheItem, exists := state.cache[cacheKey]
	if !exists {
		return t
	}

	// keep dynamic update case in mind
	if cacheItem.timer == nil && c.conf.TTLSec > 0 {
		cacheItem.timer = c.releaseCacheTimer(ctx, cacheKey)
	} else if cacheItem.timer != nil && c.conf.TTLSec == 0 {
		cacheItem.timer.Stop()
		cacheItem.timer = nil
	} else if cacheItem.timer != nil && c.conf.TTLSec > 0 {
		cacheItem.timer.Reset(time.Duration(c.conf.TTLSec) * time.Second)
	} else { // cacheItem.timer == nil && c.conf.TTLSec == 0
		// Nothing to do.
	}

	t.WithValue(cacheItem.name, cacheItem.data)

	if c.conf.FinishIfHit {
		// to add the flag to prevent loop caching only if cache hits can finish pipeline
		// otherwise here assumes follow plugins might update cached data.
		t.WithValue(fmt.Sprintf("%s_CACHED", c.Name()), true)

		t.Finish()
	}

	return t
}

////

const (
	simpleCommonCacheStateKey = "simpleCommonCacheStateKey"
)

type simpleCommonCacheItem struct {
	name  string
	data  interface{}
	timer *time.Timer
}

type simpleCommonCacheState struct {
	sync.RWMutex
	cache map[uint32]*simpleCommonCacheItem
}

func getSimpleCommonCacheState(ctx pipelines.PipelineContext, pluginName string) (*simpleCommonCacheState, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	state, err := bucket.QueryDataWithBindDefault(simpleCommonCacheStateKey,
		func() interface{} {
			return &simpleCommonCacheState{
				cache: make(map[uint32]*simpleCommonCacheItem),
			}
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to cache data: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return state.(*simpleCommonCacheState), nil
}
