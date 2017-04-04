package plugins

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"logger"
	"pipelines"
	"task"
)

type ioReaderConfig struct {
	CommonConfig
	LengthMax int64 `json:"read_length_max"` // up to 9223372036854775807 ~= 8192 Pebibyte
	Close     bool  `json:"close_after_read"`

	InputKey  string `json:"input_key"`
	OutputKey string `json:"output_key"`
}

func IOReaderConfigConfigConstructor() Config {
	return &ioReaderConfig{
		LengthMax: 1048576, // 1MiB
		Close:     true,
	}
}

func (c *ioReaderConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.InputKey, c.OutputKey = ts(c.InputKey), ts(c.OutputKey)

	if len(c.InputKey) == 0 {
		return fmt.Errorf("invalid input key")
	}

	// FIXME: why not check whether c.OutputKey is empty? Only Read?

	if c.LengthMax < 1 {
		logger.Warnf("[UNLIMITED read length has been applied, all data could be read in to memory!]")
	}

	return nil
}

type ioReader struct {
	conf *ioReaderConfig
}

func IOReaderConstructor(conf Config) (Plugin, error) {
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

	reader := input
	if r.conf.LengthMax > 0 {
		reader = io.LimitReader(reader, r.conf.LengthMax)
	}

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err, task.ResultInternalServerError, t
	}

	if len(r.conf.OutputKey) != 0 {
		t, err = task.WithValue(t, r.conf.OutputKey, data)
		if err != nil {
			return err, task.ResultInternalServerError, t
		}
	}

	if r.conf.Close {
		input1, ok := inputValue.(io.ReadCloser)
		if ok {
			err = input1.Close()
			if err != nil {
				logger.Warnf("[close io input reader faild, ignored: %s", err)
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

func (r *ioReader) Close() {
	// Nothing to do.
}
