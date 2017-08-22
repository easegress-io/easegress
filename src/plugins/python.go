package plugins

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
	"option"
)

const PYTHON_CODE_WORK_DIR = "/tmp/easegateway_python_plugin"

type pythonConfig struct {
	common.PluginCommonConfig
	Code               string `json:"code"`
	Base64             bool   `json:"base64_encoded"`
	Version            string `json:"version"`
	InputBufferPattern string `json:"input_buffer_pattern"`
	OutputKey          string `json:"output_key"`
	TimeoutSec         uint16 `json:"timeout_sec"` // up to 65535, zero means no timeout
	ExpectedExitCodes  []int  `json:"expected_exit_codes"`

	executableCode string
	cmd            string
}

func PythonConfigConstructor() plugins.Config {
	return &pythonConfig{
		TimeoutSec:        10,
		Version:           "2",
		ExpectedExitCodes: []int{0},
	}
}

func (c *pythonConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if len(c.Code) == 0 {
		return fmt.Errorf("invalid python code")
	}

	if c.Base64 {
		ec, err := base64.StdEncoding.DecodeString(c.Code)
		if err != nil {
			return fmt.Errorf("invalid base64 encoded python code")
		}
		c.executableCode = string(ec)
	} else {
		c.executableCode = c.Code
	}

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

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no code could be terminated by execution timeout!]")
	}

	ts := strings.TrimSpace
	c.OutputKey = ts(c.OutputKey)

	_, err = common.ScanTokens(c.InputBufferPattern, false, nil)
	if err != nil {
		return fmt.Errorf("invalid input buffer pattern")
	}

	os.RemoveAll(PYTHON_CODE_WORK_DIR)
	os.MkdirAll(PYTHON_CODE_WORK_DIR, 750)

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

	if !option.PluginPythonRootNamespace {
		cmd.SysProcAttr = common.SysProcAttr()
	}
	cmd.Dir = PYTHON_CODE_WORK_DIR

	// skip error check safely due to we ensured it in Prepare()
	input, _ := ReplaceTokensInPattern(t, p.conf.InputBufferPattern)

	if len(input) != 0 {
		in, err := cmd.StdinPipe()
		if err != nil {
			logger.Errorf("[prepare stdin of python command failed: %v]", err)

			t.SetError(err, task.ResultServiceUnavailable)
			return t, nil
		}

		go func() {
			defer in.Close()
			io.WriteString(in, input)
		}()
	}

	var stdOut, stdErr bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Start()
	if err != nil {
		logger.Errorf("[launch python interpreter failed: %v]", err)

		t.SetError(err, task.ResultServiceUnavailable)
		return t, nil
	}

	done := make(chan error, 0)
	defer close(done)

	go func() {
		done <- cmd.Wait()
	}()

	var timer <-chan time.Time

	if p.conf.TimeoutSec > 0 {
		timer = time.After(time.Duration(p.conf.TimeoutSec) * time.Second)
	} else {
		timer1 := make(chan time.Time, 0)
		defer close(timer1)

		timer = timer1
	}

	select {
	case err := <-done:
		if stdErr.Len() > 0 {
			logger.Warnf("[python code wrote stderr:\n%s\n]", stdErr.String())
		}

		t = handleResult(err, p.conf.ExpectedExitCodes, stdOut, p.conf.OutputKey, t)
	case <-timer:
		cmd.Process.Kill()
		<-done // wait goroutine exits

		logger.Errorf("[execute python code timeout, terminated]")

		err := fmt.Errorf("python code execution timeout")
		t.SetError(err, task.ResultServiceUnavailable)
	case <-t.Cancel():
		cmd.Process.Kill()
		<-done // wait goroutine exits

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

////

func handleResult(err error, expectedExitCodes []int,
	out bytes.Buffer, outputKey string, t task.Task) task.Task {

	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()

				logger.Debugf("[execute python code exit code: %d]", exitCode)
			}
		} else {
			logger.Errorf("[execute python code failed: %v]", err)

			t.SetError(err, task.ResultServiceUnavailable)
			return t
		}
	}

	if pythonExitCodeExpected(exitCode, expectedExitCodes) {
		if len(outputKey) != 0 {
			t, err = task.WithValue(t, outputKey, out.Bytes())
			if err != nil {
				t.SetError(err, task.ResultInternalServerError)
			}
		}
	} else {
		err := fmt.Errorf("python code responded with unexpected exit code (%d)", exitCode)
		t.SetError(err, task.ResultServiceUnavailable)
	}

	return t
}

func pythonExitCodeExpected(code int, expectedExitCodes []int) bool {
	if len(expectedExitCodes) == 0 {
		return true
	}

	for _, expected := range expectedExitCodes {
		if code == expected {
			return true
		}
	}

	return false
}
