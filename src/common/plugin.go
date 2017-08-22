package common

import (
	"fmt"
	"strings"
)

type PluginCommonConfig struct {
	Name string `json:"plugin_name"`
}

func (c *PluginCommonConfig) PluginName() string {
	return c.Name
}

func (c *PluginCommonConfig) Prepare(pipelineNames []string) error {
	c.Name = strings.TrimSpace(c.Name)
	if len(c.Name) == 0 {
		return fmt.Errorf("invalid plugin name")
	}

	return nil
}
