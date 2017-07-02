package pipelines

import (
	"fmt"
	"strings"
)

type Pipeline interface {
	Name() string
	Run() error
	Stop()
	Close()
}

////

type Config interface {
	PipelineName() string
	PluginNames() []string
	Parallelism() uint16
	CrossPipelineRequestBacklog() uint16
	Prepare() error
}

////

type CommonConfig struct {
	Name                              string   `json:"pipeline_name"`
	Plugins                           []string `json:"plugin_names"`
	ParallelismCount                  uint16   `json:"parallelism"`                    // up to 65535
	CrossPipelineRequestBacklogLength uint16   `json:"cross_pipeline_request_backlog"` // up to 65535
}

func (c *CommonConfig) PipelineName() string {
	return c.Name
}

func (c *CommonConfig) PluginNames() []string {
	return c.Plugins
}

func (c *CommonConfig) Parallelism() uint16 {
	return c.ParallelismCount
}

func (c *CommonConfig) CrossPipelineRequestBacklog() uint16 {
	return c.CrossPipelineRequestBacklogLength
}

func (c *CommonConfig) Prepare() error {
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

// Pipelines register authority

var (
	PIPELINE_TYPES = map[string]interface{}{
		"LinearPipeline": nil,
	}
)

func ValidType(t string) bool {
	_, exists := PIPELINE_TYPES[t]
	return exists
}

func GetAllTypes() []string {
	types := make([]string, 0)
	for t := range PIPELINE_TYPES {
		types = append(types, t)
	}
	return types
}
