package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/hexdecteam/easegateway-types/plugins"

	"common"
	"logger"
)

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
	pluginConstructor plugins.Constructor
	configConstructor plugins.ConfigConstructor
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

func GetConstructor(t string) (plugins.Constructor, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].pluginConstructor, nil
}

func GetConfig(t string) (plugins.Config, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].configConstructor(), nil
}

// Out-tree plugin type loading

type GetTypeNames func() ([]string, error)
type GetPluginConstructor func() (plugins.Constructor, error)
type GetPluginConfigConstructor func() (plugins.ConfigConstructor, error)

func LoadOutTreePluginTypes() error {
	logger.Debugf("[load all out-tree plugin types]")

	err := os.MkdirAll(common.PLUGIN_HOME_DIR, 0700)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	count := 0

	err = filepath.Walk(common.PLUGIN_HOME_DIR, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			logger.Warnf("[access %s failed, out-tree plugin types skipped in the file: %v]", path, err)
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		typeNames, failedTypeName, err := loadOutTreePluginTypes(path)
		if err != nil {
			if failedTypeName == "" {
				logger.Errorf("[load out-tree plugin types from %s failed, skipped: %v]", path, err)
			} else {
				logger.Errorf("[load out-tree plugin type %s from %s failed: %v]",
					failedTypeName, path, err)
			}
		}

		for _, name := range typeNames {
			logger.Debugf("[out-tree plugin type %s is loaded from %s successfully]", name, path)
			count++
		}

		return nil
	})

	if err != nil {
		logger.Errorf("[load out-tree plugin types failed: %v]", err)
		return err
	}

	logger.Infof("[out-tree plugin types are loaded successfully (total=%d)]", count)
	return nil
}

func loadOutTreePluginTypes(path string) ([]string, string, error) {
	ret := make([]string, 0)

	p, err := plugin.Open(path)
	if err != nil {
		return ret, "", err
	}

	f, err := p.Lookup("GetTypeNames")
	if err != nil {
		return ret, "", err
	}

	getTypeNames, ok := f.(GetTypeNames)
	if !ok {
		return ret, "", fmt.Errorf("invalid plugin type names definition")
	}

	names, err := getTypeNames()
	if err != nil {
		return ret, "", err
	}

	for _, name := range names {
		// check if definition is existing
		_, exists := PLUGIN_ENTRIES[name]
		if exists {
			logger.Warnf("[plugin type %s definied in %s is conflicting, skipped]", name, path)
			continue
		}

		// plugin constructor
		f, err := p.Lookup(fmt.Sprintf("Get%sConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConstructor, ok := f.(GetPluginConstructor)
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin constructor definition")
		}

		var c plugins.Constructor
		var e error

		if common.PanicToErr(func() { c, e = getConstructor() }, &err) {
			return ret, name, err
		} else if e != nil {
			return ret, name, e
		}

		// plugin configuration constructor
		f, err = p.Lookup(fmt.Sprintf("Get%sConfigConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConfigConstructor, ok := f.(GetPluginConfigConstructor)
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin config constructor definition")
		}

		var cc plugins.ConfigConstructor
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
