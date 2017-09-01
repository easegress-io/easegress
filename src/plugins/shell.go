package plugins

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hexdecteam/easegateway-types/plugins"

	"common"
	"logger"
	"option"
)

const SHELL_PLUGIN_WORK_DIR = "/tmp/easegateway_shell_plugin"

type shellConfig struct {
	interpreterRunnerConfig
	Type string `json:"type"`

	cmd string
}

func ShellConfigConstructor() plugins.Config {
	c := &shellConfig{
		interpreterRunnerConfig: newInterpreterRunnerConfig("shell", SHELL_PLUGIN_WORK_DIR),
		Type: "sh",
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

func ShellConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*shellConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *shellConfig got %T", conf)
	}

	base, err := newInterpreterRunner(&c.interpreterRunnerConfig)
	if err != nil {
		return nil, err
	}

	p := &shell{
		interpreterRunner: base,
		conf:              c,
	}

	p.interpreterRunner.executor = p

	return p, nil
}

func (p *shell) command(code string) *exec.Cmd {
	ret := exec.Command(p.conf.cmd, "-c", code)

	if !option.PluginShellRootNamespace {
		ret.SysProcAttr = common.SysProcAttr()
	}

	return ret
}
