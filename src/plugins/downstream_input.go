package plugins

import (
	"fmt"

	"logger"
	"pipelines"
	"task"
)

type downstreamInputConfig struct {
	CommonConfig

	ResponseDataKeys []string `json:"response_data_keys"`
}

func DownstreamInputConfigConstructor() Config {
	return &downstreamInputConfig{}
}

func (c *downstreamInputConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	return nil
}

type downstreamInput struct {
	conf *downstreamInputConfig
}

func DownstreamInputConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*downstreamInputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *downstreamInputConfig got %T", conf)
	}

	return &downstreamInput{
		conf: c,
	}, nil
}

func (d *downstreamInput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (d *downstreamInput) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	request := ctx.ClaimCrossPipelineRequest()
	if request == nil {
		// request was closed by downstream before any upstream handles it, ignore safely
		return t, nil
	}

	if request.UpstreamPipelineName() != ctx.PipelineName() {
		logger.Errorf("[BUG: downstream pipeline %s sends the request of "+
			"cross pipeline request to the wrong upstream %s]",
			request.DownstreamPipelineName(), ctx.PipelineName())

		t.SetError(fmt.Errorf("upstream received wrong downstream request"),
			task.ResultInternalServerError)
		return t, nil
	}

	for k, v := range request.Data() {
		t1, err := task.WithValue(t, k, v)
		if err != nil {
			t.SetError(err, task.ResultInternalServerError)
			return t, nil
		}

		t = t1
	}

	respondDownstreamRequest := func(t1 task.Task, _ task.TaskStatus) {
		data := make(map[interface{}]interface{})
		for _, key := range d.conf.ResponseDataKeys {
			data[key] = t1.Value(key)
		}

		response := &pipelines.UpstreamResponse{
			UpstreamPipelineName: d.Name(),
			Data:                 data,
			TaskError:            t1.Error(),
			TaskResultCode:       t1.ResultCode(),
		}

		ret := request.Respond(response, t1.Cancel())
		if ret {
			logger.Debugf("[respond downstream pipeline %s successfully]", request.DownstreamPipelineName())
		} else {
			logger.Errorf("[respond downstream pipeline %s failed]", request.DownstreamPipelineName())
		}

		t1.DeleteFinishedCallback(fmt.Sprintf("%s-respondDownstreamRequest", d.Name()))
	}

	t.AddFinishedCallback(fmt.Sprintf("%s-respondDownstreamRequest", d.Name()), respondDownstreamRequest)

	return t, nil
}

func (d *downstreamInput) Name() string {
	return d.conf.PluginName()
}

func (d *downstreamInput) Close() {
	// Nothing to do.
}
