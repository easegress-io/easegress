package plugins

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
)

type pythonConfig struct {
	CommonConfig
	Code       string `json:"code"`
	Base64     bool   `json:"base64_encoded"`
	Version    string `json:"version"`
	InputKey   string `json:"input_key"`
	OutputKey  string `json:"output_key"`
	TimeoutSec uint16 `json:"timeout_sec"` // up to 65535, zero means no timeout

	executableCode string
	cmd            string
}

func PythonConfigConstructor() plugins.Config {
	return &pythonConfig{
		TimeoutSec: 10,
		Version:    "2",
	}
}

func (c *pythonConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if len(c.Code) == 0 {
		return fmt.Errorf("invalid python code")
	}

	if c.Base64 {
		ec, err := base64.StdEncoding.DecodeString(c.Code)
		if err != nil {
			return fmt.Errorf("invalid base64 encoded python code", err)
		}
		c.executableCode = string(ec)
	} else {
		c.executableCode = c.Code
	}

	// NOTICE: Perhaps support minor version such as 2.7, 3.6, etc in future.
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
		logger.Warnf("[python interpreter (version=%s) is not ready, python plugin will runs unsucessfullly!]",
			c.Version)
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no code could be terminated by execution timeout!]")
	}

	ts := strings.TrimSpace
	c.InputKey = ts(c.InputKey)
	c.OutputKey = ts(c.OutputKey)

	return nil
}

type python struct {
	conf *pythonConfig
}

func PythonConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*pythonConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *pythonConfig got %T", conf)
	}

	return &python{
		conf: c,
	}, nil
}

func (p *python) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (p *python) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	cmd := exec.Command(p.conf.cmd, "-c", p.conf.executableCode)
	cmd.SysProcAttr = common.SysProcAttr()

	if len(p.conf.InputKey) != 0 {
		in, err := cmd.StdinPipe()
		if err != nil {
			logger.Errorf("[prepare stdin of python command failed: %v]", err)

			t.SetError(err, task.ResultServiceUnavailable)
			return t, nil
		}

		go func() {
			defer in.Close()
			io.WriteString(in, task.ToString(t.Value(p.conf.InputKey)))
		}()
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	done := make(chan error, 0)

	go func() {
		err := cmd.Start()
		if err != nil {
			logger.Errorf("[launch python interpreter failed: %v]", err)
			done <- err
			return
		}

		err = cmd.Wait()
		if err != nil {
			logger.Errorf("[execute python code failed: %v]", err)
			done <- err
		}

		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.SetError(err, task.ResultServiceUnavailable)
		} else if len(p.conf.OutputKey) != 0 {
			t, err = task.WithValue(t, p.conf.OutputKey, out.Bytes())
			if err != nil {
				t.SetError(err, task.ResultInternalServerError)
			}
		}
	case <-time.After(time.Duration(p.conf.TimeoutSec) * time.Second):
		cmd.Process.Kill()

		logger.Errorf("[execute python code timeout, terminated]")

		err := fmt.Errorf("python code execution timeout")
		t.SetError(err, task.ResultServiceUnavailable)
	case <-t.Cancel():
		cmd.Process.Kill()

		err := fmt.Errorf("task is cancelled by %s", t.CancelCause())
		t.SetError(err, task.ResultTaskCancelled)
	}

	return t, nil
}

func (p *python) Name() string {
	return p.conf.PluginName()
}

func (p *python) Close() {
	// Nothing to do.
}
