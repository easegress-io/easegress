package plugins

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
)

type ioReaderConfig struct {
	common.PluginCommonConfig
	LengthMax    int64  `json:"read_length_max"` // up to 9223372036854775807 ~= 8192 Pebibyte
	Close        bool   `json:"close_after_read"`
	Base64Coding string `json:"base64_coding"`

	InputKey  string `json:"input_key"`
	OutputKey string `json:"output_key"`
}

func ioReaderConfigConfigConstructor() plugins.Config {
	return &ioReaderConfig{
		LengthMax: 1048576, // 1MiB
		Close:     true,
	}
}

func (c *ioReaderConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.Base64Coding = ts(c.Base64Coding)
	c.InputKey, c.OutputKey = ts(c.InputKey), ts(c.OutputKey)

	if !common.StrInSlice(c.Base64Coding, []string{"", "encode", "decode"}) {
		return fmt.Errorf("invalid base64 coding flag")
	}

	if c.LengthMax < 1 {
		logger.Warnf("[UNLIMITED read length has been applied, all data could be read in to memory!]")
	}

	if len(c.InputKey) == 0 {
		return fmt.Errorf("invalid input key")
	}

	return nil
}

type ioReader struct {
	conf *ioReaderConfig
}

func ioReaderConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*ioReaderConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *ioReaderConfig got %T", conf)
	}

	return &ioReader{
		conf: c,
	}, nil
}

func (r *ioReader) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (r *ioReader) read(t task.Task) (error, task.TaskResultCode, task.Task) {
	inputValue := t.Value(r.conf.InputKey)
	input, ok := inputValue.(io.Reader)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", r.conf.InputKey, inputValue),
			task.ResultMissingInput, t
	}

	reader1 := common.NewInterruptibleReader(input)
	var reader io.Reader = reader1
	if r.conf.LengthMax > 0 {
		reader = io.LimitReader(reader, r.conf.LengthMax)
	}

	d := make(chan []byte, 1)
	e := make(chan error, 1)

	defer close(d)
	defer close(e)

	go func() {
		if r.conf.Base64Coding == "decode" {
			reader = base64.NewDecoder(base64.StdEncoding, reader)
		}

		data, err := ioutil.ReadAll(reader)
		if err != nil {
			e <- err
			return
		}

		if r.conf.Base64Coding == "encode" {
			buff := bytes.NewBuffer(nil)
			encoder := base64.NewEncoder(base64.StdEncoding, buff)
			_, err = encoder.Write(data)
			encoder.Close()

			if err != nil {
				e <- err
				return
			}

			data = buff.Bytes()
		}

		d <- data
	}()

	var data []byte
	select {
	case data = <-d:
		reader1.Close()
	case err := <-e:
		reader1.Close()
		return err, task.ResultBadInput, t
	case <-t.Cancel():
		reader1.Cancel()
		return fmt.Errorf("task is cancelled by %s", t.CancelCause()), task.ResultTaskCancelled, t
	}

	if len(r.conf.OutputKey) != 0 {
		var err error
		t, err = task.WithValue(t, r.conf.OutputKey, data)
		if err != nil {
			return err, task.ResultInternalServerError, t
		}
	}

	if r.conf.Close {
		input1, ok := inputValue.(io.Closer)
		if ok {
			err := input1.Close()
			if err != nil {
				logger.Warnf("[close io input reader failed, ignored: %v", err)
			}
		}
	}

	return nil, t.ResultCode(), t
}

func (r *ioReader) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := r.read(t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (r *ioReader) Name() string {
	return r.conf.PluginName()
}

func (r *ioReader) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (r *ioReader) Close() {
	// Nothing to do.
}
