package easemonitor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
)

type jsonGidExtractorConfig struct {
	common.PluginCommonConfig
	GidKey  string `json:"gid_key"`
	DataKey string `json:"data_key"`
}

func JSONGidExtractorConfigConstructor() plugins.Config {
	return &jsonGidExtractorConfig{}
}

func (c *jsonGidExtractorConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.GidKey, c.DataKey = ts(c.GidKey), ts(c.DataKey)

	if len(c.GidKey) == 0 {
		return fmt.Errorf("invalid gid key")
	}

	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}

type jsonGidExtractor struct {
	conf *jsonGidExtractorConfig
}

func JSONGidExtractorConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, error) {
	c, ok := conf.(*jsonGidExtractorConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, fmt.Errorf("config type want *jsonGidExtractorConfig got %T", conf)
	}

	return &jsonGidExtractor{
		conf: c,
	}, plugins.ProcessPlugin, nil
}

func (e *jsonGidExtractor) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (e *jsonGidExtractor) extract(t task.Task) (error, task.TaskResultCode, task.Task) {
	type straw struct {
		System      string `json:"system"`
		Application string `json:"application"`
		Instance    string `json:"instance"`
		HostIPv4    string `json:"hostipv4"`
		Hostname    string `json:"hostname"`
	}

	dataValue := t.Value(e.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", e.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	var s straw
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err, task.ResultBadInput, t
	}

	gid := strings.Join([]string{s.System, s.Application, s.Instance, s.HostIPv4, s.Hostname}, "")

	t.WithValue(e.conf.GidKey, gid)

	return nil, t.ResultCode(), t
}

func (e *jsonGidExtractor) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := e.extract(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return nil
}

func (e *jsonGidExtractor) Name() string {
	return e.conf.PluginName()
}

func (e *jsonGidExtractor) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (e *jsonGidExtractor) Close() {
	// Nothing to do.
}
