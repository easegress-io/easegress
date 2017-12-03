package plugins

import (
	"fmt"
	"strings"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
)

type simpleCommonMockConfig struct {
	common.PluginCommonConfig
	PluginConcerned         string   `json:"plugin_concerned"`
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
	c.PluginConcerned = ts(c.PluginConcerned)
	c.MockTaskDataKey = ts(c.MockTaskDataKey)
	c.MockTaskDataValue = ts(c.MockTaskDataValue)

	if len(c.PluginConcerned) == 0 {
		return fmt.Errorf("invalid plugin name")
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
		getTaskRecoveryFuncInSimpleCommonMock(m.conf.PluginConcerned,
			m.conf.taskErrorCodesConcerned, m.conf.FinishTask,
			m.conf.MockTaskDataKey, m.conf.MockTaskDataValue))
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

func getTaskRecoveryFuncInSimpleCommonMock(pluginConcerned string,
	taskErrorCodesConcerned []task.TaskResultCode, finishTask bool, mockTaskDataKey,
	mockTaskDataValue string) task.TaskRecovery {

	return func(t task.Task, errorPluginName string) (bool, bool) {
		if errorPluginName != pluginConcerned {
			return false, false
		}
		recovered := false
		for _, code := range taskErrorCodesConcerned {
			if t.ResultCode() == code {
				recovered = true
				break
			}
		}
		if recovered {
			t.WithValue(mockTaskDataKey, mockTaskDataValue)
		}

		return recovered, finishTask
	}
}
