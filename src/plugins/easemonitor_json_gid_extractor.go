package plugins

import (
	"encoding/json"
	"fmt"
	"strings"

	"pipelines"
	"task"
)

type easeMonitorJSONGidExtractorConfig struct {
	CommonConfig
	GidKey  string `json:"gid_key"`
	DataKey string `json:"data_key"`
}

func EaseMonitorJSONGidExtractorConfigConstructor() Config {
	return &easeMonitorJSONGidExtractorConfig{}
}

func (c *easeMonitorJSONGidExtractorConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
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

type easeMonitorJSONGidExtractor struct {
	conf *easeMonitorJSONGidExtractorConfig
}

func EaseMonitorJSONGidExtractorConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*easeMonitorJSONGidExtractorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *easeMonitorJSONGidExtractorConfig got %T", conf)
	}

	return &easeMonitorJSONGidExtractor{
		conf: c,
	}, nil
}

func (e *easeMonitorJSONGidExtractor) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (e *easeMonitorJSONGidExtractor) extract(t task.Task) (error, task.TaskResultCode, task.Task) {
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

	t, err = task.WithValue(t, e.conf.GidKey, gid)
	if err != nil {
		return err, task.ResultInternalServerError, t
	}

	return nil, t.ResultCode(), t
}

func (e *easeMonitorJSONGidExtractor) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := e.extract(t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (e *easeMonitorJSONGidExtractor) Name() string {
	return e.conf.PluginName()
}

func (e *easeMonitorJSONGidExtractor) Close() {
	// Nothing to do.
}
