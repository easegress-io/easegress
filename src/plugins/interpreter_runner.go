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
)

type interpreterRunnerConfig struct {
	common.PluginCommonConfig
	Code               string `json:"code"`
	Base64             bool   `json:"base64_encoded"`
	InputBufferPattern string `json:"input_buffer_pattern"`
	OutputKey          string `json:"output_key"`
	TimeoutSec         uint16 `json:"timeout_sec"` // up to 65535, zero means no timeout
	ExpectedExitCodes  []int  `json:"expected_exit_codes"`

	interpreterName string
	workDir         string
	executableCode  string
}

func newInterpreterRunnerConfig(interpreterName string, workDir string) interpreterRunnerConfig {
	return interpreterRunnerConfig{
		TimeoutSec:        10,
		ExpectedExitCodes: []int{0},

		interpreterName: strings.TrimSpace(interpreterName),
		workDir:         workDir,
	}
}

func (c *interpreterRunnerConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if len(c.Code) == 0 {
		return fmt.Errorf("invalid %s code", c.interpreterName)
	}

	if c.Base64 {
		ec, err := base64.StdEncoding.DecodeString(c.Code)
		if err != nil {
			return fmt.Errorf("invalid base64 encoded %s code", c.interpreterName)
		}
		c.executableCode = string(ec)
	} else {
		c.executableCode = c.Code
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no %s code could be terminated by execution timeout!]",
			c.interpreterName)
	}

	ts := strings.TrimSpace
	c.OutputKey = ts(c.OutputKey)

	_, err = common.ScanTokens(c.InputBufferPattern, false, nil)
	if err != nil {
		return fmt.Errorf("invalid input buffer pattern")
	}

	os.RemoveAll(c.workDir)
	os.MkdirAll(c.workDir, 750)

	return nil
}

type interpreterExecutor interface {
	command(code string) *exec.Cmd
}

type interpreterRunner struct {
	executor interpreterExecutor
	conf     *interpreterRunnerConfig
}

func newInterpreterRunner(conf plugins.Config) (*interpreterRunner, error) {
	c, ok := conf.(*interpreterRunnerConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *interpreterRunnerConfig got %T", conf)
	}

	p := &interpreterRunner{
		conf: c,
	}

	p.executor = p

	return p, nil
}

func (r *interpreterRunner) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (r *interpreterRunner) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	cmd := r.executor.command(r.conf.executableCode)
	if cmd == nil {
		logger.Errorf("[BUG: %s interpreter did not provide valid command, skip to execution]",
			r.conf.interpreterName)
		return t, nil
	}

	cmd.Dir = r.conf.workDir

	// skip error check safely due to we ensured it in Prepare()
	input, _ := ReplaceTokensInPattern(t, r.conf.InputBufferPattern)

	if len(input) != 0 {
		in, err := cmd.StdinPipe()
		if err != nil {
			logger.Errorf("[prepare stdin of command of %s interpreter failed: %v]", r.conf.interpreterName, err)

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
		logger.Errorf("[launch %s interpreter failed: %v]", r.conf.interpreterName, err)

		t.SetError(err, task.ResultServiceUnavailable)
		return t, nil
	}

	done := make(chan error, 0)

	go func() {
		done <- cmd.Wait()
	}()

	var timer <-chan time.Time

	if r.conf.TimeoutSec > 0 {
		timer = time.After(time.Duration(r.conf.TimeoutSec) * time.Second)
	} else {
		timer1 := make(chan time.Time, 0)
		defer close(timer1)

		timer = timer1
	}

	select {
	case err := <-done:
		close(done)

		if stdErr.Len() > 0 {
			logger.Warnf("[%s code wrote stderr:\n%s\n]", r.conf.interpreterName, stdErr.String())
		}

		t = handleInterpreterResult(r.conf.interpreterName, err, r.conf.ExpectedExitCodes, stdOut, r.conf.OutputKey, t)
	case <-timer:
		cmd.Process.Kill()

		go func() { // Process might block on such shell like `sh`
			<-done // wait goroutine exits
			close(done)
		}()

		logger.Errorf("[execute %s code timeout, terminated]", r.conf.interpreterName)

		err := fmt.Errorf("%s code execution timeout", r.conf.interpreterName)
		t.SetError(err, task.ResultServiceUnavailable)
	case <-t.Cancel():
		cmd.Process.Kill()

		go func() { // Process might block on such shell like `sh`
			<-done // wait goroutine exits
			close(done)
		}()

		err := fmt.Errorf("task is cancelled by %s", t.CancelCause())
		t.SetError(err, task.ResultTaskCancelled)
	}

	return t, nil
}

func (r *interpreterRunner) Name() string {
	return r.conf.PluginName()
}

func (r *interpreterRunner) Close() {
	// Nothing to do.
}

func (r *interpreterRunner) command(code string) *exec.Cmd {
	// implements by special plugin
	return nil
}

////

func handleInterpreterResult(interpreterName string, err error, expectedExitCodes []int,
	out bytes.Buffer, outputKey string, t task.Task) task.Task {

	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()

				logger.Debugf("[execute %s code exit code: %d]", interpreterName, exitCode)
			}
		} else {
			logger.Errorf("[execute %s code failed: %v]", interpreterName, err)

			t.SetError(err, task.ResultServiceUnavailable)
			return t
		}
	}

	if interpreterExitCodeExpected(exitCode, expectedExitCodes) {
		if len(outputKey) != 0 {
			t, err = task.WithValue(t, outputKey, out.Bytes())
			if err != nil {
				t.SetError(err, task.ResultInternalServerError)
			}
		}
	} else {
		err := fmt.Errorf("%s code exited with unexpected code (%d)", interpreterName, exitCode)
		t.SetError(err, task.ResultServiceUnavailable)
	}

	return t
}

func interpreterExitCodeExpected(exitCode int, expectedExitCodes []int) bool {
	if len(expectedExitCodes) == 0 {
		return true
	}

	for _, expected := range expectedExitCodes {
		if exitCode == expected {
			return true
		}
	}

	return false
}
