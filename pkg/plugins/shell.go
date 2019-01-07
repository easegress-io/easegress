package plugins

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

const SHELL_PLUGIN_WORK_DIR = "/tmp/easegateway_shell_plugin"

type shellConfig struct {
	interpreterRunnerConfig
	Type string `json:"type"`

	cmd string
}

func shellConfigConstructor() Config {
	c := &shellConfig{
		interpreterRunnerConfig: newInterpreterRunnerConfig("shell", SHELL_PLUGIN_WORK_DIR),
		Type:                    "sh",
	}

	c.ExpectedExitCodes = []int{0}

	return c
}

func (c *shellConfig) Prepare(pipelineNames []string) error {
	err := c.interpreterRunnerConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	c.Type = strings.TrimSpace(c.Type)

	switch c.Type {
	case "sh":
		fallthrough
	case "bash":
		fallthrough
	case "zsh":
		c.cmd = c.Type
	default:
		return fmt.Errorf("invalid shell type")
	}

	cmd := exec.Command(c.cmd, "-c", "")
	if cmd.Run() != nil {
		logger.Warnf("[shell interpreter (type=%s) is not ready, shell plugin will runs unsuccessfully!]",
			c.Type)
	}

	return nil
}

type shell struct {
	*interpreterRunner
	conf *shellConfig
}

func shellConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*shellConfig)
	if !ok {
		return nil, ProcessPlugin, false, fmt.Errorf(
			"config type want *shellConfig got %T", conf)
	}

	base, singleton, err := newInterpreterRunner(&c.interpreterRunnerConfig)
	if err != nil {
		return nil, ProcessPlugin, singleton, err
	}

	p := &shell{
		interpreterRunner: base,
		conf:              c,
	}

	p.interpreterRunner.executor = p

	return p, ProcessPlugin, singleton, nil
}

func (p *shell) command(code string) *exec.Cmd {
	ret := exec.Command(p.conf.cmd, "-c", code)

	if !option.Global.PluginShellRootNamespace {
		ret.SysProcAttr = common.SysProcAttr()
	}

	return ret
}
