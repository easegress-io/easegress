package plugins

import (
	"fmt"
	"strings"

	"logger"
	"pipelines"
	"task"
)

type simpleCommonMockConfig struct {
	CommonConfig
	PluginConcerned        string `json:"plugin_concerned"`
	TaskErrorCodeConcerned string `json:"task_error_code_concerned"`
	// TODO: Supports multiple key and value pairs
	MockTaskDataKey   string `json:"mock_task_data_key"`
	MockTaskDataValue string `json:"mock_task_data_value"`

	taskErrorCodeConcerned task.TaskResultCode
}

func SimpleCommonMockConfigConstructor() Config {
	return &simpleCommonMockConfig{
		TaskErrorCodeConcerned: "ResultFlowControl",
	}
}

func (c *simpleCommonMockConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
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

	if !task.ValidResultCodeName(c.TaskErrorCodeConcerned) {
		return fmt.Errorf("invalid task error code")
	} else {
		c.taskErrorCodeConcerned = task.ResultCodeValue(c.TaskErrorCodeConcerned)
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

func SimpleCommonMockConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*simpleCommonMockConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *simpleCommonMockConfig got %T", conf)
	}

	m := &simpleCommonMock{
		conf: c,
	}

	return m, nil
}

func (m *simpleCommonMock) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (m *simpleCommonMock) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	t.AddRecoveryFunc("mockBrokenTaskOutput",
		getTaskRecoveryFuncInSimpleCommonMock(m.conf.PluginConcerned, m.conf.taskErrorCodeConcerned,
			m.conf.MockTaskDataKey, m.conf.MockTaskDataValue))
	return t, nil
}

func (m *simpleCommonMock) Name() string {
	return m.conf.PluginName()
}

func (m *simpleCommonMock) Close() {
	// Nothing to do.
}

////

func getTaskRecoveryFuncInSimpleCommonMock(pluginConcerned string, taskErrorCodeConcerned task.TaskResultCode,
	mockTaskDataKey, mockTaskDataValue string) task.TaskRecovery {

	return func(t task.Task, errorPluginName string) (bool, task.Task) {
		if errorPluginName != pluginConcerned || t.ResultCode() != taskErrorCodeConcerned {
			return false, t
		}

		t1, err := task.WithValue(t, mockTaskDataKey, mockTaskDataValue)
		if err != nil {
			logger.Warnf("[BUG: supply mock data %s to plugin %s failed, ignored: %s]",
				mockTaskDataKey, pluginConcerned, err)
			return false, t
		}

		return true, t1
	}
}
