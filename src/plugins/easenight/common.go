package easenight

import (
	"plugins/easenight/manifest"

	"github.com/hexdecteam/easegateway-types/task"

	"common"
)

type CommonEaseNightPlugin struct {
	common.PluginCommonConfig
}

func NewCommon() *CommonEaseNightPlugin {
	return &CommonEaseNightPlugin{}
}

func (p *CommonEaseNightPlugin) GetBeforeRunCommand(m *manifest.Plugin, t *task.Task) *string {
	return nil
}

func (p *CommonEaseNightPlugin) GetAfterRunCommand(m *manifest.Plugin, t *task.Task) *string {
	return nil
}

func (p *CommonEaseNightPlugin) GetRunCommand(m *manifest.Plugin, t *task.Task) string {
	var userCode string
	scriptName := m.GetScript()
	if scriptName != "" {
		userCode = GetRunRepoScriptCommand(scriptName)
	} else {
		userCode = m.GetShell()
	}
	return GetTaskCommonScript(t, userCode)
}

func (p *CommonEaseNightPlugin) GetRequiredSettings() []string {
	return []string{
		manifest.PUSH_BY,
		manifest.PUSH_ID,
		manifest.PUSH_MESSAGE,
		manifest.GITHUB_USERNAME,
		manifest.GITHUB_PASSWORD,
	}
}
