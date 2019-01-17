package plugins

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"

	metrics "github.com/rcrowley/go-metrics"
)

type PluginCommonConfig struct {
	Name string `json:"plugin_name"`
}

func (c *PluginCommonConfig) PluginName() string {
	return c.Name
}

func (c *PluginCommonConfig) Prepare(pipelineNames []string) error {
	c.Name = strings.TrimSpace(c.Name)
	if len(c.Name) == 0 {
		return fmt.Errorf("invalid plugin name")
	}

	return nil
}

///
type ThroughputStatisticType int

const (
	ThroughputRate1 ThroughputStatisticType = iota
	ThroughputRate5
	ThroughputRate15
)

type ThroughputStatistic struct {
	// EWMA is thread safe
	ewma   metrics.EWMA
	done   chan struct{}
	closed bool
}

func NewThroughputStatistic(typ ThroughputStatisticType) *ThroughputStatistic {
	var ewma metrics.EWMA
	switch typ {
	case ThroughputRate1:
		ewma = metrics.NewEWMA1()
	case ThroughputRate5:
		ewma = metrics.NewEWMA5()
	case ThroughputRate15:
		ewma = metrics.NewEWMA15()
	}

	done := make(chan struct{})
	tickFunc := func(ewma metrics.EWMA) {
		ticker := time.NewTicker(time.Duration(5) * time.Second)
		for {
			select {
			case <-ticker.C:
				// ewma.Tick ticks the clock to update the moving average.  It assumes it is called
				// every five seconds.
				ewma.Tick()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}
	go tickFunc(ewma)

	return &ThroughputStatistic{
		ewma: ewma,
		done: done,
	}
}

func (t *ThroughputStatistic) Get() (float64, error) {
	return t.ewma.Rate(), nil
}

func (t *ThroughputStatistic) Update(n int64) {
	t.ewma.Update(n)
}

func (t *ThroughputStatistic) Close() error { // io.Closer stub
	if !t.closed {
		t.closed = true
		close(t.done)
	}
	return nil
}

type PluginType uint8

const (
	UnknownType PluginType = iota
	SourcePlugin
	SinkPlugin
	ProcessPlugin
)

// Plugin needs to cover follow rules:
//
// 1. Run(task.Task) method returns error only if
//    a) the plugin needs reconstruction, e.g. backend failure causes local client object invalidation;
//    b) the task has been cancelled by pipeline after running plugin is updated dynamically, task will
//    re-run on updated plugin;
//    The error caused by user input should be updated to task instead.
// 2. Should be implemented as stateless and be re-entry-able (idempotency) on the same task, a plugin
//    instance could be used in different pipeline or parallel running instances of same pipeline.
// 3. Prepare(pipelines.PipelineContext) guarantees it will be called on the same pipeline context against
//    the same plugin instance only once before executing Run(task.Task) on the pipeline.
type Plugin interface {
	Prepare(ctx pipelines.PipelineContext)
	Run(ctx pipelines.PipelineContext, t task.Task) error
	Name() string
	CleanUp(ctx pipelines.PipelineContext)
	Close()
}

type Constructor func(conf Config) (Plugin, PluginType, bool, error)

type Config interface {
	PluginName() string
	Prepare(pipelineNames []string) error
}

type ConfigConstructor func() Config

////

type SizedReadCloser interface {
	io.ReadCloser
	// Size indicates the available bytes length of reader
	// negative value means available bytes length unknown
	Size() int64
}

type HTTPCtx interface {
	RequestHeader() Header
	ResponseHeader() Header
	RemoteAddr() string
	BodyReadCloser() SizedReadCloser
	DumpRequest() (string, error)

	// return nil if concrete type doesn't support CloseNotifier
	CloseNotifier() http.CloseNotifier

	// SetStatusCode sends an HTTP response header with the provided
	// status code.
	//
	// If SetStatusCode is not called explicitly, the first call to Write
	// will trigger an implicit SetStatusCode(http.StatusOK).
	// Thus explicit calls to SetStatusCode are mainly used to send
	// error codes.
	SetStatusCode(statusCode int)

	// SetContentLength should be called before calling SetStatusCode or Write.
	// Otherwise it does't have any effect.
	SetContentLength(len int64)

	// Write writes the data to the connection as part of an HTTP reply.
	Write(p []byte) (int, error)
}

type Header interface {
	// Get methods
	Proto() string
	Method() string
	Get(k string) string
	Host() string
	Scheme() string
	// path (relative paths may omit leading slash)
	// for example: "search" in "http://www.google.com:80/search?q=megaease#title
	Path() string
	// full url, for example: http://www.google.com?s=megaease#title
	FullURI() string
	QueryString() string
	ContentLength() int64
	// VisitAll calls f for each header.
	VisitAll(f func(k, v string))

	CopyTo(dst Header) error

	// Set sets the given 'key: value' header.
	Set(k, v string)

	// Del deletes header with the given key.
	Del(k string)

	// Add adds the given 'key: value' header.
	// Multiple headers with the same key may be added with this function.
	// Use Set for setting a single header for the given key.
	Add(k, v string)

	// Set sets the given 'key: value' header.
	SetContentLength(len int64)
}

type HTTPType int8

type HTTPHandler func(ctx HTTPCtx, urlParams map[string]string, routeDuration time.Duration)

type HTTPURLPattern struct {
	Scheme   string
	Host     string
	Port     string
	Path     string
	Query    string
	Fragment string
}

type HTTPMuxEntry struct {
	HTTPURLPattern
	Method   string
	Priority uint32
	Instance Plugin
	Headers  map[string][]string
	Handler  HTTPHandler
}

type HTTPMux interface {
	ServeHTTP(ctx HTTPCtx)
	AddFunc(ctx pipelines.PipelineContext, entryAdding *HTTPMuxEntry) error
	AddFuncs(ctx pipelines.PipelineContext, entriesAdding []*HTTPMuxEntry) error
	DeleteFunc(ctx pipelines.PipelineContext, entryDeleting *HTTPMuxEntry)
	DeleteFuncs(ctx pipelines.PipelineContext) []*HTTPMuxEntry
}

const (
	HTTP_SERVER_MUX_BUCKET_KEY                  = "HTTP_SERVER_MUX_BUCKET_KEY"
	HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY = "HTTP_SERVER_PIPELINE_ROUTE_TABLE_BUCKET_KEY"
	HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY        = "HTTP_SERVER_GONE_NOTIFIER_BUCKET_KEY"
)

// Plugins Register Authority

type pluginEntry struct {
	pluginConstructor Constructor
	configConstructor ConfigConstructor
}

var (
	PLUGIN_ENTRIES = map[string]pluginEntry{
		// generic plugins
		"HTTPServer": {
			httpServerConstructor, HTTPServerConfigConstructor},
		"HTTPInput": {
			httpInputConstructor, HTTPInputConfigConstructor},
		"JSONValidator": {
			jsonValidatorConstructor, jsonValidatorConfigConstructor},
		"KafkaOutput": {
			kafkaOutputConstructor, kafkaOutputConfigConstructor},
		"ThroughputRateLimiter": {
			throughputRateLimiterConstructor, throughputRateLimiterConfigConstructor},
		"LatencyLimiter": {
			latencyLimiterConstructor, latencyLimiterConfigConstructor},
		"ServiceCircuitBreaker": {
			serviceCircuitBreakerConstructor, serviceCircuitBreakerConfigConstructor},
		"StaticProbabilityLimiter": {
			staticProbabilityLimiterConstructor, staticProbabilityLimiterConfigConstructor},
		"NoMoreFailureLimiter": {
			noMoreFailureLimiterConstructor, noMoreFailureLimiterConfigConstructor},
		"SimpleCommonMock": {
			simpleCommonMockConstructor, simpleCommonMockConfigConstructor},
		"HTTPHeaderCounter": {
			httpHeaderCounterConstructor, httpHeaderCounterConfigConstructor},
		"HTTPOutput": {
			httpOutputConstructor, HTTPOutputConfigConstructor},
		"IOReader": {
			ioReaderConstructor, ioReaderConfigConfigConstructor},
		"SimpleCommonCache": {
			simpleCommonCacheConstructor, simpleCommonCacheConfigConstructor},
		"UpstreamOutput": {
			upstreamOutputConstructor, upstreamOutputConfigConstructor},
		"DownstreamInput": {
			downstreamInputConstructor, downstreamInputConfigConstructor},
		"Python": {
			pythonConstructor, pythonConfigConstructor},
		"Shell": {
			shellConstructor, shellConfigConstructor},

		// Ease Monitor product dedicated plugins
		"EaseMonitorGraphiteGidExtractor": {
			GraphiteGidExtractorConstructor, GraphiteGidExtractorConfigConstructor},
		"EaseMonitorGraphiteValidator": {
			GraphiteValidatorConstructor, GraphiteValidatorConfigConstructor},
		"EaseMonitorJSONGidExtractor": {
			JSONGidExtractorConstructor, JSONGidExtractorConfigConstructor},
	}

	PLUGIN_TYPES []string
)

func init() {
	for t := range PLUGIN_ENTRIES {
		PLUGIN_TYPES = append(PLUGIN_TYPES, t)
	}
}

func ValidType(t string) bool {
	_, ok := PLUGIN_ENTRIES[t]
	return ok
}

func GetAllTypes() []string {
	types := make([]string, 0, len(PLUGIN_ENTRIES))
	for t := range PLUGIN_ENTRIES {
		types = append(types, t)
	}
	return types
}

func GetConstructor(t string) (Constructor, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].pluginConstructor, nil
}

func GetConfig(t string) (Config, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].configConstructor(), nil
}

func GetConstructorConfig(t string) (Constructor, Config, error) {
	if !ValidType(t) {
		return nil, nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].pluginConstructor, PLUGIN_ENTRIES[t].configConstructor(), nil
}

// Out-tree plugin type loading

func LoadOutTreePluginTypes() error {
	logger.Debugf("load all out-tree plugin types")

	err := os.MkdirAll(option.Global.CertDir, 0700)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	count := 0

	err = filepath.Walk(option.Global.CertDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			logger.Warnf("access %s failed, out-tree plugin types skipped in the file: %v", path, err)
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		typeNames, failedTypeName, err := loadOutTreePluginTypes(path)
		if err != nil {
			if failedTypeName == "" {
				logger.Errorf("load out-tree plugin types from %s failed, skipped: %v", path, err)
			} else {
				logger.Errorf("load out-tree plugin type %s from %s failed, skipped: %v",
					failedTypeName, path, err)
			}
		}

		for _, name := range typeNames {
			logger.Debugf("out-tree plugin type %s is loaded from %s successfully", name, path)
			count++
		}

		return nil
	})

	if err != nil {
		logger.Errorf("load out-tree plugin types failed: %v", err)
		return err
	}

	logger.Infof("out-tree plugin types are loaded successfully (total=%d)", count)
	return nil
}

func loadOutTreePluginTypes(path string) ([]string, string, error) {
	ret := make([]string, 0)

	p, err := plugin.Open(path)
	if err != nil {
		return ret, "", err
	}

	f, err := p.Lookup("GetPluginTypeNames")
	if err != nil {
		return ret, "", err
	}

	getTypeNames, ok := f.(func() ([]string, error))
	if !ok {
		return ret, "", fmt.Errorf("invalid plugin type names definition (%T)", f)
	}

	names, err := getTypeNames()
	if err != nil {
		return ret, "", err
	}

	for _, name := range names {
		// check if definition is existing
		_, exists := PLUGIN_ENTRIES[name]
		if exists {
			logger.Warnf("plugin type %s definied in %s is conflicting, skipped", name, path)
			continue
		}

		// plugin constructor
		f, err := p.Lookup(fmt.Sprintf("Get%sPluginConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConstructor, ok := f.(func() (Constructor, error))
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin constructor definition (%T)", f)
		}

		var c Constructor
		var e error

		if common.PanicToErr(func() { c, e = getConstructor() }, &err) {
			return ret, name, err
		} else if e != nil {
			return ret, name, e
		}

		// plugin configuration constructor
		f, err = p.Lookup(fmt.Sprintf("Get%sPluginConfigConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConfigConstructor, ok := f.(func() (ConfigConstructor, error))
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin config constructor definition (%T)", f)
		}

		var cc ConfigConstructor
		e = nil

		if common.PanicToErr(func() { cc, e = getConfigConstructor() }, &err) {
			return ret, name, err
		} else if e != nil {
			return ret, name, e
		}

		// register
		PLUGIN_ENTRIES[name] = pluginEntry{
			pluginConstructor: c,
			configConstructor: cc,
		}
	}

	return ret, "", nil
}

// Plugins utils

func ReplaceTokensInPattern(t task.Task, pattern string) (string, error) {
	visitor := func(pos int, token string) (bool, string) {
		var ret string

		v := t.Value(token)
		if v != nil {
			ret = task.ToString(v, option.Global.PluginIODataFormatLengthLimit)
		}

		// always do replacement even it is empty (no value found in the task with a certain data key)
		return true, ret
	}

	removeEscapeChar := strings.Contains(pattern, common.TOKEN_ESCAPE_CHAR)

	return common.ScanTokens(pattern, removeEscapeChar, visitor)
}

// limiter related utils

const limiterInboundThroughputRate1Key = "limiterInboundThroughputRate1"
const limiterFlowControlledThroughputRate1Key = "limiterFlowControlledThroughputRate1"

func getInboundThroughputRate1(ctx pipelines.PipelineContext,
	pluginName string) (*ThroughputStatistic, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	rate, err := bucket.QueryDataWithBindDefault(limiterInboundThroughputRate1Key, func() interface{} {
		return NewThroughputStatistic(ThroughputRate1)
	})
	if err != nil {
		logger.Warnf("BUG: query in-throughput rate data for pipeline %s failed, "+
			"ignored statistic: %v", ctx.PipelineName(), err)
		return nil, err
	}

	return rate.(*ThroughputStatistic), nil
}

func getFlowControlledThroughputRate1(ctx pipelines.PipelineContext,
	pluginName string) (*ThroughputStatistic, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	rate, err := bucket.QueryDataWithBindDefault(limiterFlowControlledThroughputRate1Key, func() interface{} {
		return NewThroughputStatistic(ThroughputRate1)
	})
	if err != nil {
		logger.Warnf("BUG: query flow controlled throughput rate data for pipeline %s failed, "+
			"ignored statistic: %v", ctx.PipelineName(), err)
		return nil, err
	}

	return rate.(*ThroughputStatistic), nil
}

func getFlowControlledPercentage(ctx pipelines.PipelineContext, pluginName string) (int, error) {
	rate, err := getFlowControlledThroughputRate1(ctx, pluginName)
	if err != nil {
		return 0, err
	}
	pluginFlowControlledRate, err := rate.Get()
	if err != nil {
		return 0, err
	}
	rate, err = getInboundThroughputRate1(ctx, pluginName)
	if err != nil {
		return 0, err
	}
	pluginInThroughputRate, err := rate.Get()
	if pluginInThroughputRate == 0 { // avoid divide by zero
		return 0, nil
	}
	return int(100.0 * pluginFlowControlledRate / pluginInThroughputRate), nil
}

func registerPluginIndicatorForLimiter(ctx pipelines.PipelineContext, pluginName, pluginInstanceId string) {
	_, err := ctx.Statistics().RegisterPluginIndicator(pluginName, pluginInstanceId,
		"THROUGHPUT_RATE_LAST_1MIN_FLOWCONTROLLED", "Flow controlled throughput rate of the plugin in last 1 minute.",
		func(pluginName, indicatorName string) (interface{}, error) {
			rate, err := getFlowControlledThroughputRate1(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return rate.Get()
		})
	if err != nil {
		logger.Warnf("BUG: register plugin indicator for pipeline %s plugin %s failed: %v",
			ctx.PipelineName(), pluginName, err)
	}

	// We don't use limiter plugin's THROUGHPUT_RATE_LAST_1MIN_ALL
	// because it indicates the throughput rate after applying flow control
	_, err = ctx.Statistics().RegisterPluginIndicator(pluginName, pluginInstanceId,
		"THROUGHPUT_RATE_LAST_1MIN_INBOUND", "All inbound throughput rate of the plugin in last 1 minute.",
		func(pluginName, indicatorName string) (interface{}, error) {
			rate, err := getInboundThroughputRate1(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return rate.Get()
		})
	if err != nil {
		logger.Warnf("BUG: register plugin indicator for pipeline %s plugin %s failed: %v",
			ctx.PipelineName(), pluginName, err)
	}
}

func updateFlowControlledThroughputRate(ctx pipelines.PipelineContext, pluginName string) error {
	flowControlledRate1, err := getFlowControlledThroughputRate1(ctx, pluginName)
	if err != nil {
		return err
	}
	flowControlledRate1.Update(1)
	return nil
}

func updateInboundThroughputRate(ctx pipelines.PipelineContext, pluginName string) error {
	inRate1, err := getInboundThroughputRate1(ctx, pluginName)
	if err != nil {
		return err
	}
	inRate1.Update(1)
	return nil
}
