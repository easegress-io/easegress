package plugins

import (
	"fmt"
	"strings"

	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

type simpleCommonMockConfig struct {
	common.PluginCommonConfig
	PluginsConcerned        []string `json:"plugins_concerned"`
	PluginTypesConcerned    []string `json:"plugin_types_concerned"`
	TaskErrorCodesConcerned []string `json:"task_error_codes_concerned"`
	FinishTask              bool     `json:"finish_task"`
	// TODO: Supports multiple key and value pairs
	MockTaskDataKey   string `json:"mock_task_data_key"`
	MockTaskDataValue string `json:"mock_task_data_value"`

	taskErrorCodesConcerned []task.TaskResultCode
}

func simpleCommonMockConfigConstructor() plugins.Config {
	return &simpleCommonMockConfig{}
}

func (c *simpleCommonMockConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.MockTaskDataKey = ts(c.MockTaskDataKey)
	c.MockTaskDataValue = ts(c.MockTaskDataValue)

	if len(c.PluginsConcerned) == 0 && len(c.PluginTypesConcerned) == 0 {
		return fmt.Errorf("either concerned plugins or types needs to be configured")
	}

	if len(c.PluginsConcerned) > 0 && len(c.PluginTypesConcerned) > 0 {
		return fmt.Errorf("both concerned plugins and types are configured")
	}

	if len(c.PluginTypesConcerned) > 0 {
		types := GetAllTypes()
		for _, typ := range c.PluginTypesConcerned {
			if !common.StrInSlice(typ, types) {
				return fmt.Errorf("invalid concerned plugin types")
			}
		}
	}

	if len(c.TaskErrorCodesConcerned) == 0 {
		return fmt.Errorf("empty task error codes")
	}
	for _, code := range c.TaskErrorCodesConcerned {
		if !task.ValidResultCodeName(code) {
			return fmt.Errorf("invalid task error codes")
		} else {
			c.taskErrorCodesConcerned = append(c.taskErrorCodesConcerned, task.ResultCodeValue(code))
		}
	}

	if len(c.MockTaskDataKey) == 0 {
		return fmt.Errorf("invalid mock task data key")
	}

	return nil
}

////

type simpleCommonMock struct {
	conf *simpleCommonMockConfig
}

func simpleCommonMockConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*simpleCommonMockConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, false, fmt.Errorf(
			"config type want *simpleCommonMockConfig got %T", conf)
	}

	m := &simpleCommonMock{
		conf: c,
	}

	return m, plugins.ProcessPlugin, false, nil
}

func (m *simpleCommonMock) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (m *simpleCommonMock) Run(ctx pipelines.PipelineContext, t task.Task) error {
	t.AddRecoveryFunc(fmt.Sprintf("mockBrokenPlugin-%s", m.Name()),
		getTaskRecoveryFuncInSimpleCommonMock(m.conf))
	return nil
}

func (m *simpleCommonMock) Name() string {
	return m.conf.PluginName()
}

func (m *simpleCommonMock) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (m *simpleCommonMock) Close() {
	// Nothing to do.
}

////

func getTaskRecoveryFuncInSimpleCommonMock(conf *simpleCommonMockConfig) task.TaskRecovery {

	return func(t task.Task, errorPluginName, errorPluginType string) (bool, bool) {
		if len(conf.PluginsConcerned) > 0 && !common.StrInSlice(errorPluginName, conf.PluginsConcerned) ||
			len(conf.PluginTypesConcerned) > 0 && !common.StrInSlice(errorPluginType, conf.PluginTypesConcerned) {
			return false, false
		}
		recovered := false
		for _, code := range conf.taskErrorCodesConcerned {
			if t.ResultCode() == code {
				recovered = true
				break
			}
		}
		if recovered {
			t.WithValue(conf.MockTaskDataKey, conf.MockTaskDataValue)
		}

		return recovered, conf.FinishTask
	}
}
