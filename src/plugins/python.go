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

const PYTHON_PLUGIN_WORK_DIR = "/tmp/easegateway_python_plugin"

type pythonConfig struct {
	interpreterRunnerConfig
	Version string `json:"version"`

	cmd string
}

func pythonConfigConstructor() plugins.Config {
	c := &pythonConfig{
		interpreterRunnerConfig: newInterpreterRunnerConfig("python", PYTHON_PLUGIN_WORK_DIR),
		Version:                 "2",
	}

	c.ExpectedExitCodes = []int{0}

	return c
}

func (c *pythonConfig) Prepare(pipelineNames []string) error {
	err := c.interpreterRunnerConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	c.Version = strings.TrimSpace(c.Version)

	// NOTE(longyun): Perhaps support minor version such as 2.7, 3.6, etc in future.
	switch c.Version {
	case "2":
		c.cmd = "python2"
	case "3":
		c.cmd = "python3"
	default:
		return fmt.Errorf("invalid python version")
	}

	cmd := exec.Command(c.cmd, "-c", "")
	if cmd.Run() != nil {
		logger.Warnf("[python interpreter (version=%s) is not ready, python plugin will runs unsuccessfully!]",
			c.Version)
	}

	return nil
}

type python struct {
	*interpreterRunner
	conf *pythonConfig
}

func pythonConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, error) {
	c, ok := conf.(*pythonConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, fmt.Errorf("config type want *pythonConfig got %T", conf)
	}

	base, err := newInterpreterRunner(&c.interpreterRunnerConfig)
	if err != nil {
		return nil, plugins.ProcessPlugin, err
	}

	p := &python{
		interpreterRunner: base,
		conf:              c,
	}

	p.interpreterRunner.executor = p

	return p, plugins.ProcessPlugin, nil
}

func (p *python) command(code string) *exec.Cmd {
	ret := exec.Command(p.conf.cmd, "-c", code)

	if !option.PluginPythonRootNamespace {
		ret.SysProcAttr = common.SysProcAttr()
	}

	return ret
}
