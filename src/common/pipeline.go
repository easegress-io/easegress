package common

import (
	"fmt"
	"strings"
)

type PipelineCommonConfig struct {
	Name                              string   `json:"pipeline_name"`
	Plugins                           []string `json:"plugin_names"`
	ParallelismCount                  uint16   `json:"parallelism"`                    // up to 65535
	CrossPipelineRequestBacklogLength uint16   `json:"cross_pipeline_request_backlog"` // up to 65535
}

func (c *PipelineCommonConfig) PipelineName() string {
	return c.Name
}

func (c *PipelineCommonConfig) PluginNames() []string {
	return c.Plugins
}

func (c *PipelineCommonConfig) Parallelism() uint16 {
	return c.ParallelismCount
}

func (c *PipelineCommonConfig) CrossPipelineRequestBacklog() uint16 {
	return c.CrossPipelineRequestBacklogLength
}

func (c *PipelineCommonConfig) Prepare() error {
	if len(strings.TrimSpace(c.PipelineName())) == 0 {
		return fmt.Errorf("invalid pipeline name")
	}

	if len(c.PluginNames()) == 0 {
		return fmt.Errorf("pipeline is empty")
	}

	if c.Parallelism() < 1 {
		return fmt.Errorf("invalid pipeline parallelism")
	}

	return nil
}
