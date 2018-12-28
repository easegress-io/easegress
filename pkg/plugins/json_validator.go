package plugins

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"

	"github.com/xeipuuv/gojsonschema"
)

type jsonValidatorConfig struct {
	PluginCommonConfig
	Schema  string `json:"schema"`
	Base64  bool   `json:"base64_encoded"`
	DataKey string `json:"data_key"`

	schemaObj *gojsonschema.Schema
}

func jsonValidatorConfigConstructor() Config {
	return &jsonValidatorConfig{}
}

func (c *jsonValidatorConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	schema := c.Schema
	if c.Base64 {
		ec, err := base64.StdEncoding.DecodeString(c.Schema)
		if err != nil {
			return fmt.Errorf("invalid base64 encoded schema")
		}
		schema = string(ec)
	}

	loader := gojsonschema.NewBytesLoader([]byte(schema))
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

func jsonValidatorConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*jsonValidatorConfig)
	if !ok {
		return nil, ProcessPlugin, false, fmt.Errorf(
			"config type want *jsonValidatorConfig got %T", conf)
	}

	return &jsonValidator{
		conf: c,
	}, ProcessPlugin, false, nil
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
			errs = append(errs, fmt.Sprintf("%d: %v", i+1, err))
		}

		return fmt.Errorf(strings.Join(errs, ", ")), task.ResultBadInput, t
	}

	return nil, t.ResultCode(), t
}

func (v *jsonValidator) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := v.validate(t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return nil
}

func (v *jsonValidator) Name() string {
	return v.conf.PluginName()
}

func (v *jsonValidator) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (v *jsonValidator) Close() {
	// Nothing to do.
}
