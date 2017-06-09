package plugins

import (
	"fmt"
	"strings"

	"pipelines"
	"task"
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
//    Under current implementation, a plugin couldn't be used in different pipeline but there is no
//    guarantee this limitation is existing in future release.
// 3. Prepare(pipelines.PipelineContext) guarantees it will be called on the same pipeline context against
//    the same plugin instance only once before executing Run(task.Task) on the pipeline.
type Plugin interface {
	Prepare(ctx pipelines.PipelineContext)
	Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error)
	Name() string
	Close()
}

type Constructor func(conf Config) (Plugin, error)
type ConfigConstructor func() Config

type Config interface {
	PluginName() string
	Prepare(pipelineNames []string) error
}

type CommonConfig struct {
	Name string `json:"plugin_name"`
}

func (c *CommonConfig) PluginName() string {
	return c.Name
}

func (c *CommonConfig) Prepare(pipelineNames []string) error {
	c.Name = strings.TrimSpace(c.Name)
	if len(c.Name) == 0 {
		return fmt.Errorf("invalid plugin name")
	}

	return nil
}

// Plugins Register Authority

type pluginEntry struct {
	pluginConstructor Constructor
	configConstructor ConfigConstructor
}

var (
	PLUGIN_ENTRIES = map[string]pluginEntry{
		// generic plugins
		"HTTPInput": {
			HTTPInputConstructor, HTTPInputConfigConstructor},
		"JSONValidator": {
			JSONValidatorConstructor, JSONValidatorConfigConstructor},
		"KafkaOutput": {
			KafkaOutputConstructor, KafkaOutputConfigConstructor},
		"ThroughputRateLimiter": {
			ThroughputRateLimiterConstructor, ThroughputRateLimiterConfigConstructor},
		"LatencyWindowLimiter": {
			LatencyWindowLimiterConstructor, LatencyWindowLimiterConfigConstructor},
		"ServiceCircuitBreaker": {
			ServiceCircuitBreakerConstructor, ServiceCircuitBreakerConfigConstructor},
		"StaticProbabilityLimiter": {
			StaticProbabilityLimiterConstructor, StaticProbabilityLimiterConfigConstructor},
		"NoMoreFailureLimiter": {
			NoMoreFailureLimiterConstructor, NoMoreFailureLimiterConfigConstructor},
		"SimpleCommonMock": {
			SimpleCommonMockConstructor, SimpleCommonMockConfigConstructor},
		"HTTPHeaderCounter": {
			HTTPHeaderCounterConstructor, HTTPHeaderCounterConfigConstructor},
		"HTTPOutput": {
			HTTPOutputConstructor, HTTPOutputConfigConstructor},
		"IOReader": {
			IOReaderConstructor, IOReaderConfigConfigConstructor},
		"SimpleCommonCache": {
			SimpleCommonCacheConstructor, SimpleCommonCacheConfigConstructor},
		"UpstreamOutput": {
			UpstreamOutputConstructor, UpstreamOutputConfigConstructor},
		"DownstreamInput": {
			DownstreamInputConstructor, DownstreamInputConfigConstructor},

		// Ease Monitor product dedicated plugins
		"EaseMonitorProtoAdaptor": {
			EaseMonitorProtoAdaptorConstructor, EaseMonitorProtoAdaptorConfigConstructor},
		"EaseMonitorGraphiteGidExtractor": {
			EaseMonitorGraphiteGidExtractorConstructor, EaseMonitorGraphiteGidExtractorConfigConstructor},
		"EaseMonitorGraphiteValidator": {
			EaseMonitorGraphiteValidatorConstructor, EaseMonitorGraphiteValidatorConfigConstructor},
		"EaseMonitorJSONGidExtractor": {
			EaseMonitorJSONGidExtractorConstructor, EaseMonitorJSONGidExtractorConfigConstructor},
	}
)

func ValidType(t string) bool {
	_, ok := PLUGIN_ENTRIES[t]
	return ok
}

func GetAllTypes() []string {
	types := make([]string, 0)
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
