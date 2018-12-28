package plugins

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"
)

type downstreamInputConfig struct {
	PluginCommonConfig

	ResponseDataKeys []string `json:"response_data_keys"`
}

func downstreamInputConfigConstructor() Config {
	return &downstreamInputConfig{}
}

func (c *downstreamInputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	return nil
}

type downstreamInput struct {
	conf *downstreamInputConfig
}

func downstreamInputConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*downstreamInputConfig)
	if !ok {
		return nil, SourcePlugin, false, fmt.Errorf(
			"config type want *downstreamInputConfig got %T", conf)
	}

	return &downstreamInput{
		conf: c,
	}, SourcePlugin, false, nil
}

func (d *downstreamInput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (d *downstreamInput) Run(ctx pipelines.PipelineContext, t task.Task) error {
	request := ctx.ClaimCrossPipelineRequest(t.Cancel())
	if t.CancelCause() != nil {
		t.SetError(fmt.Errorf("task is cancelled by %s", t.CancelCause()), task.ResultTaskCancelled)
		return t.Error()
	}

	if request == nil {
		// request was closed by downstream before any upstream handles it, ignore safely
		return nil
	}

	if request.UpstreamPipelineName() != ctx.PipelineName() {
		logger.Errorf("[BUG: downstream pipeline %s sends the request of "+
			"cross pipeline request to the wrong upstream %s]",
			request.DownstreamPipelineName(), ctx.PipelineName())

		t.SetError(fmt.Errorf("upstream received wrong downstream request"), task.ResultInternalServerError)
		return nil
	}

	for k, v := range request.Data() {
		t.WithValue(k, v)
	}

	respondDownstreamRequest := func(t1 task.Task, _ task.TaskStatus) {
		data := make(map[string]interface{})
		for _, key := range d.conf.ResponseDataKeys {
			data[key] = t1.Value(key)
		}

		response := &pipelines.UpstreamResponse{
			UpstreamPipelineName: ctx.PipelineName(),
			Data:                 data,
			TaskError:            t1.Error(),
			TaskResultCode:       t1.ResultCode(),
		}

		err := request.Respond(response, t1.Cancel())
		if err != nil {
			logger.Warnf("[respond downstream pipeline %s failed: %v]",
				request.DownstreamPipelineName(), err)
		}
	}

	t.AddFinishedCallback(fmt.Sprintf("%s-respondDownstreamRequest", d.Name()), respondDownstreamRequest)

	return nil
}

func (d *downstreamInput) Name() string {
	return d.conf.PluginName()
}

func (d *downstreamInput) CleanUp(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (d *downstreamInput) Close() {
	// Nothing to do.
}
