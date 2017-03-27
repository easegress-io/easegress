package config

import (
	"plugins"
)

type PluginSpec struct {
	Type        string              `json:"type"`
	Config      interface{}         `json:"config"`
	Constructor plugins.Constructor `json:"-"`
}

type PipelineSpec struct {
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}
