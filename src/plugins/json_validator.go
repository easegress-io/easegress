package plugins

import (
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"

	"pipelines"
	"task"
)

type jsonValidatorConfig struct {
	CommonConfig

	Schema  string `json:"schema"`
	DataKey string `json:"data_key"`

	schemaObj *gojsonschema.Schema
}

func JSONValidatorConfigConstructor() Config {
	return &jsonValidatorConfig{}
}

func (c *jsonValidatorConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
	if err != nil {
		return err
	}

	loader := gojsonschema.NewBytesLoader([]byte(c.Schema))
	c.schemaObj, err = gojsonschema.NewSchema(loader)
	if err != nil {
		return fmt.Errorf("invalid schema: %v", err)
	}

	ts := strings.TrimSpace
	c.DataKey = ts(c.DataKey)

	if len(c.DataKey) == 0 {
		return fmt.Errorf("invalid data key")
	}

	return nil
}

type jsonValidator struct {
	conf *jsonValidatorConfig
}

func JSONValidatorConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*jsonValidatorConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *jsonValidatorConfig got %T", conf)
	}

	return &jsonValidator{
		conf: c,
	}, nil
}

func (v *jsonValidator) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *jsonValidator) validate(t task.Task) (error, task.TaskResultCode, task.Task) {
	// defensive programming
	if v.conf.schemaObj == nil {
		return fmt.Errorf("schema not found"), task.ResultInternalServerError, t
	}

	dataValue := t.Value(v.conf.DataKey)
	data, ok := dataValue.([]byte)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", v.conf.DataKey, dataValue),
			task.ResultMissingInput, t
	}

	res, err := v.conf.schemaObj.Validate(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return err, task.ResultBadInput, t
	}

	if !res.Valid() {
		var errs []string
		for i, err := range res.Errors() {
			errs = append(errs, fmt.Sprintf("%d: %s", i+1, err))
		}

		return fmt.Errorf(strings.Join(errs, ", ")), task.ResultBadInput, t
	}

	return nil, t.ResultCode(), t
}

func (v *jsonValidator) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := v.validate(t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (v *jsonValidator) Name() string {
	return v.conf.PluginName()
}

func (v *jsonValidator) Close() {
	// Nothing to do.
}
