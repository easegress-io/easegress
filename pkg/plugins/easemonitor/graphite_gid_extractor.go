package easemonitor

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

type graphiteGidExtractorConfig struct {
	common.PluginCommonConfig
	GidKey  string `json:"gid_key"`
	DataKey string `json:"data_key"`
}

func GraphiteGidExtractorConfigConstructor() plugins.Config {
	return &graphiteGidExtractorConfig{}
}

func (c *graphiteGidExtractorConfig) Prepare(pipelineNames []string) error {
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

type graphiteGidExtractor struct {
	conf *graphiteGidExtractorConfig
}

func GraphiteGidExtractorConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*graphiteGidExtractorConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, false, fmt.Errorf(
			"config type want *graphiteGidExtractorConfig got %T", conf)
	}

	return &graphiteGidExtractor{
		conf: c,
	}, plugins.ProcessPlugin, false, nil
}

func (e *graphiteGidExtractor) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (e *graphiteGidExtractor) extract(t task.Task) (error, task.TaskResultCode, task.Task) {
	dataValue := t.Value(e.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", e.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	s := bufio.NewScanner(bytes.NewReader(data))
	if !s.Scan() {
		return fmt.Errorf("unexpected EOF"), task.ResultBadInput, t
	}

	fields := common.GraphiteSplit(s.Text(), ".", "#")
	if len(fields) != 4 {
		return fmt.Errorf("graphite data want 4 fields('#'-splitted) got %v",
			len(fields)), task.ResultBadInput, t
	}

	// system application instance hostipv4 hostname
	gid := strings.Join([]string{fields[0], "", fields[1], fields[2], fields[3]}, "")

	t.WithValue(e.conf.GidKey, gid)

	return nil, t.ResultCode(), t
}

func (e *graphiteGidExtractor) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := e.extract(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return nil
}

func (e *graphiteGidExtractor) Name() string {
	return e.conf.PluginName()
}

func (e *graphiteGidExtractor) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (e *graphiteGidExtractor) Close() {
	// Nothing to do.
}
