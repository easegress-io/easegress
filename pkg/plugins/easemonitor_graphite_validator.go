package plugins

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"
)

type graphiteValidatorConfig struct {
	PluginCommonConfig
	DataKey string `json:"data_key"`
}

func GraphiteValidatorConfigConstructor() Config {
	return &graphiteValidatorConfig{}
}

func (c *graphiteValidatorConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.DataKey = ts(c.DataKey)

	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}

type graphiteValidator struct {
	conf *graphiteValidatorConfig
}

func GraphiteValidatorConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*graphiteValidatorConfig)
	if !ok {
		return nil, ProcessPlugin, false, fmt.Errorf(
			"config type want *graphiteValidatorConfig got %T", conf)
	}

	return &graphiteValidator{
		conf: c,
	}, ProcessPlugin, false, nil
}

func (v *graphiteValidator) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *graphiteValidator) validate(t task.Task) (error, task.TaskResultCode, task.Task) {
	dataValue := t.Value(v.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", v.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	if len(data) == 0 {
		return fmt.Errorf("graphite data got EOF"), task.ResultBadInput, t
	}

	s := bufio.NewScanner(bytes.NewReader(data))

	for s.Scan() {
		text := s.Text()
		fields := common.GraphiteSplit(text, ".", "#")
		if len(fields) != 4 {
			return fmt.Errorf("graphite data want 4 fields('#'-splitted) got %v", len(fields)),
				task.ResultBadInput, t
		}
	}

	return nil, t.ResultCode(), t
}

func (v *graphiteValidator) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := v.validate(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return nil
}

func (v *graphiteValidator) Name() string {
	return v.conf.PluginName()
}

func (v *graphiteValidator) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *graphiteValidator) Close() {
	// Nothing to do.
}
