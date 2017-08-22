/**
 * Created by g7tianyi on 14/08/2017
 */

package manifest

import (
	"encoding/json"

	"plugins/easenight/common"
)

const (
	PIPELINE_MODE_EASE_GATEWAY = 1
	PIPELINE_MODE_ANSIBLE      = 2 // DEFAULT
	PIPELINE_MODE_AGENT        = 3
)

type Pipeline struct {
	Plugins []*Plugin `json:"plugins"`
}

func NewPipeline() *Pipeline {
	return &Pipeline{make([]*Plugin, 0)}
}

func (p *Pipeline) GetPlugins() []*Plugin {
	return p.Plugins
}

func (p *Pipeline) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Plugins)
}

func (p *Pipeline) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &p.Plugins)
}

func (p *Pipeline) MarshalYAML() (string, error) {
	return common.Marshal(p.Plugins)
}

func (p *Pipeline) UnmarshalYAML(yamlStr string) error {
	return common.Unmarshal(yamlStr, &p.Plugins)
}
